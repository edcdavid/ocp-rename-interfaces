package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/deliedit/ocp-rename-interfaces/pkg/machineconfig"
	"github.com/spf13/cobra"
)

var (
	macAddresses   string
	namePolicy     string
	interfaceNames string
	kubeconfig     string
	output         string
	apply          bool
	mcName         string
	vendorID       string
	modelID        string
	refIfName      string
	node           string
)

var rootCmd = &cobra.Command{
	Use:   "ocp-rename-interfaces",
	Short: "Generate and apply OpenShift MachineConfig for network interface renaming",
	Long: `A tool to generate MachineConfig resources for renaming network interfaces
in OpenShift clusters using systemd .link files. Supports both NamePolicy-based
renaming and explicit interface naming.`,
	RunE: runRoot,
}

func init() {
	rootCmd.Flags().StringVarP(&macAddresses, "macs", "m", "", "Comma-separated list of MAC addresses (e.g., aa:bb:cc:dd:ee:ff,11:22:33:44:55:66)")
	rootCmd.Flags().StringVarP(&namePolicy, "name-policy", "p", "", "Single NamePolicy scheme (e.g., slot, path, onboard, mac, keep)")
	rootCmd.Flags().StringVarP(&interfaceNames, "names", "n", "", "Comma-separated list of interface names (e.g., ptp0,ptp1). Must match number of MACs.")
	rootCmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "k", "", "Path to kubeconfig file (uses KUBECONFIG env or ~/.kube/config if not specified)")
	rootCmd.Flags().StringVarP(&output, "output", "o", "", "Output file path (prints to stdout if not specified)")
	rootCmd.Flags().BoolVarP(&apply, "apply", "a", false, "Apply the MachineConfig to the cluster")
	rootCmd.Flags().StringVar(&mcName, "mc-name", "50-interface-rename", "Name of the MachineConfig resource")
	rootCmd.Flags().StringVar(&vendorID, "vendor", "", "Vendor ID in hex format (e.g., 0x8086). Use with --model for property-based matching.")
	rootCmd.Flags().StringVar(&modelID, "model", "", "Model ID in hex format (e.g., 0x153a). Use with --vendor for property-based matching.")
	rootCmd.Flags().StringVar(&refIfName, "refIfName", "", "Reference interface name to auto-detect vendor and model IDs using udevadm")
	rootCmd.Flags().StringVar(&node, "node", "", "Node name for remote vendor/model detection via 'oc debug node'. Use with --refIfName and --kubeconfig.")
}

func Execute() error {
	return rootCmd.Execute()
}

func runRoot(cmd *cobra.Command, args []string) error {
	macs, names, policy, vendor, model, err := parseAndValidateFlags()
	if err != nil {
		return err
	}

	if apply {
		return applyToCluster(macs, names, policy, vendor, model)
	}

	return generateAndOutput(macs, names, policy, vendor, model)
}

func parseAndValidateFlags() (macs, names []string, policy, vendor, model string, err error) {
	// Handle vendor/model ID detection
	vendor = strings.TrimSpace(vendorID)
	model = strings.TrimSpace(modelID)

	// Auto-detect vendor/model from reference interface
	if refIfName != "" {
		if vendor != "" || model != "" {
			return nil, nil, "", "", "", fmt.Errorf("--refIfName cannot be used with --vendor or --model")
		}
		
		var detectedVendor, detectedModel string
		var err error
		
		// Check if we should detect from cluster node or local machine
		if node != "" {
			// Detect from cluster node using oc debug
			kubeconfigPath := getKubeconfigPath()
			if kubeconfigPath == "" {
				return nil, nil, "", "", "", fmt.Errorf("--node requires --kubeconfig or KUBECONFIG environment variable")
			}
			fmt.Printf("Detecting vendor/model from interface %s on node %s...\n", refIfName, node)
			detectedVendor, detectedModel, err = getVendorModelFromClusterNode(kubeconfigPath, node, refIfName)
			if err != nil {
				return nil, nil, "", "", "", fmt.Errorf("failed to get vendor/model from node %s interface %s: %w", node, refIfName, err)
			}
			fmt.Printf("Auto-detected from node %s interface %s: Vendor ID=%s, Model ID=%s\n", node, refIfName, detectedVendor, detectedModel)
		} else {
			// Detect from local machine
			fmt.Printf("Detecting vendor/model from local interface %s...\n", refIfName)
			detectedVendor, detectedModel, err = getVendorModelFromInterface(refIfName)
			if err != nil {
				return nil, nil, "", "", "", fmt.Errorf("failed to get vendor/model from interface %s: %w", refIfName, err)
			}
			fmt.Printf("Auto-detected from local interface %s: Vendor ID=%s, Model ID=%s\n", refIfName, detectedVendor, detectedModel)
		}
		
		vendor = detectedVendor
		model = detectedModel
	}

	// Validate vendor/model pairing
	if (vendor != "" && model == "") || (vendor == "" && model != "") {
		return nil, nil, "", "", "", fmt.Errorf("--vendor and --model must be specified together")
	}
	
	// Validate --node usage
	if node != "" && refIfName == "" {
		return nil, nil, "", "", "", fmt.Errorf("--node requires --refIfName to specify which interface to detect")
	}

	// Parse comma-separated MAC addresses
	macs = parseMACAddresses(macAddresses)

	// Check that we have at least one matching method
	if len(macs) == 0 && vendor == "" {
		return nil, nil, "", "", "", fmt.Errorf("at least one matching method must be specified: --macs or --vendor/--model")
	}

	// Parse naming options
	policy = strings.TrimSpace(namePolicy)
	if interfaceNames != "" {
		names = parseCommaSeparated(interfaceNames)
	}

	// Validate inputs
	if policy == "" && len(names) == 0 {
		return nil, nil, "", "", "", fmt.Errorf("either --name-policy or --names must be specified")
	}

	if policy != "" && len(names) > 0 {
		return nil, nil, "", "", "", fmt.Errorf("--name-policy and --names are mutually exclusive")
	}

	// If using --names with MACs, count must match MACs count
	if len(names) > 0 && len(macs) > 0 && len(names) != len(macs) {
		return nil, nil, "", "", "", fmt.Errorf("number of names (%d) must match number of MAC addresses (%d)", len(names), len(macs))
	}

	// If using vendor/model with --names, only one name is expected
	if vendor != "" && len(names) > 1 {
		return nil, nil, "", "", "", fmt.Errorf("when using --vendor/--model matching, only one interface name can be specified")
	}

	return macs, names, policy, vendor, model, nil
}

func applyToCluster(macs, names []string, policy, vendor, model string) error {
	kubeconfigPath := getKubeconfigPath()

	fmt.Printf("Using kubeconfig: %s\n", kubeconfigPath)
	fmt.Print("\nCluster information:\n")

	isSingleNode, clusterInfo, err := machineconfig.IsClusterSingleNode(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to detect cluster topology: %w", err)
	}

	fmt.Println(clusterInfo)

	if isSingleNode {
		fmt.Println("\n⚠️  Single-node or master schedulable cluster detected - will use 'master' role label")
	} else {
		fmt.Println("\n✓ Multi-node cluster detected - will use 'worker' role label")
	}

	// Generate with appropriate role
	mc, err := generateMachineConfig(isSingleNode, macs, names, policy, vendor, model)
	if err != nil {
		return err
	}

	// Display the MachineConfig
	if err := displayMachineConfig(mc); err != nil {
		return err
	}

	// Ask for confirmation
	if !confirmApply() {
		fmt.Println("Aborted.")
		return nil
	}

	// Apply to cluster
	if err := machineconfig.ApplyMachineConfig(context.Background(), kubeconfigPath, mc); err != nil {
		return fmt.Errorf("failed to apply MachineConfig: %w", err)
	}

	fmt.Printf("\n✓ MachineConfig '%s' applied successfully!\n", mc.Metadata.Name)
	fmt.Println("\nNote: The Machine Config Operator will roll out this change to the nodes.")
	fmt.Println("This may take several minutes and will cause node reboots.")

	return nil
}

func displayMachineConfig(mc *machineconfig.MachineConfig) error {
	yamlData, err := machineconfig.MarshalMachineConfig(mc)
	if err != nil {
		return fmt.Errorf("failed to marshal MachineConfig: %w", err)
	}

	const separatorLength = 80
	separator := strings.Repeat("=", separatorLength)
	fmt.Println("\n" + separator)
	fmt.Println("MachineConfig to be applied:")
	fmt.Println(separator)
	fmt.Println(string(yamlData))
	fmt.Println(separator)

	return nil
}

func confirmApply() bool {
	fmt.Print("\nDo you want to apply this MachineConfig to the cluster? (yes/no): ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "yes" || response == "y"
}

func generateAndOutput(macs, names []string, policy, vendor, model string) error {
	// Generate without applying (default to worker for file generation)
	mc, err := generateMachineConfig(false, macs, names, policy, vendor, model)
	if err != nil {
		return err
	}

	// Marshal to YAML
	yamlData, err := machineconfig.MarshalMachineConfig(mc)
	if err != nil {
		return fmt.Errorf("failed to marshal MachineConfig: %w", err)
	}

	// Output
	if output != "" {
		if err := os.WriteFile(output, yamlData, machineconfig.DefaultConfigFileMode); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("MachineConfig written to: %s\n", output)
	} else {
		fmt.Println(string(yamlData))
	}

	return nil
}

func generateMachineConfig(isSingleNode bool, macs, names []string, policy, vendor, model string) (*machineconfig.MachineConfig, error) {
	role := "worker"
	if isSingleNode {
		role = "master"
	}

	// Handle vendor/model-based matching
	if vendor != "" && model != "" {
		if len(names) > 0 {
			return machineconfig.NewMachineConfigWithPropertyAndName(mcName, role, vendor, model, names[0])
		}
		return machineconfig.NewMachineConfigWithPropertyAndPolicy(mcName, role, vendor, model, policy)
	}

	// Handle MAC-based matching
	if len(names) > 0 {
		return machineconfig.NewMachineConfigWithExplicitNames(mcName, role, macs, names)
	}

	return machineconfig.NewMachineConfigWithPolicy(mcName, role, macs, policy)
}

// parseMACAddresses parses comma-separated MAC addresses and trims whitespace
func parseMACAddresses(input string) []string {
	if input == "" {
		return []string{}
	}

	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// parseCommaSeparated parses comma-separated strings and trims whitespace
func parseCommaSeparated(input string) []string {
	if input == "" {
		return []string{}
	}

	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

func getKubeconfigPath() string {
	if kubeconfig != "" {
		return kubeconfig
	}

	if env := os.Getenv("KUBECONFIG"); env != "" {
		return env
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return homeDir + "/.kube/config"
}

// getVendorModelFromInterface executes udevadm to get vendor and model IDs from an interface
func getVendorModelFromInterface(ifName string) (vendorID, modelID string, err error) {
	// Build the udevadm command
	sysPath := fmt.Sprintf("/sys/class/net/%s", ifName)
	cmd := exec.Command("udevadm", "info", "-q", "property", "-p", sysPath)

	// Execute the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("failed to execute udevadm: %w (output: %s)", err, string(output))
	}

	// Parse the output
	return parseUdevadmOutput(string(output), ifName)
}

// getVendorModelFromClusterNode executes oc debug node to get vendor and model IDs from a cluster node
func getVendorModelFromClusterNode(kubeconfigPath, nodeName, ifName string) (vendorID, modelID string, err error) {
	// Build the oc debug node command
	// Command: oc debug node/<node> --kubeconfig=<path> -- chroot /host udevadm info -q property -p /sys/class/net/<ifName>
	sysPath := fmt.Sprintf("/sys/class/net/%s", ifName)
	
	args := []string{
		"debug",
		fmt.Sprintf("node/%s", nodeName),
		fmt.Sprintf("--kubeconfig=%s", kubeconfigPath),
		"--",
		"chroot",
		"/host",
		"udevadm",
		"info",
		"-q",
		"property",
		"-p",
		sysPath,
	}

	cmd := exec.Command("oc", args...)

	// Execute the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("failed to execute oc debug node: %w (output: %s)", err, string(output))
	}

	// Parse the output
	return parseUdevadmOutput(string(output), ifName)
}

// parseUdevadmOutput parses udevadm output to extract vendor and model IDs
func parseUdevadmOutput(output, ifName string) (vendorID, modelID string, err error) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ID_VENDOR_ID=") {
			vendorID = strings.TrimPrefix(line, "ID_VENDOR_ID=")
			// Ensure it has 0x prefix
			if !strings.HasPrefix(vendorID, "0x") {
				vendorID = "0x" + vendorID
			}
		}
		if strings.HasPrefix(line, "ID_MODEL_ID=") {
			modelID = strings.TrimPrefix(line, "ID_MODEL_ID=")
			// Ensure it has 0x prefix
			if !strings.HasPrefix(modelID, "0x") {
				modelID = "0x" + modelID
			}
		}
	}

	// Validate we found both IDs
	if vendorID == "" || modelID == "" {
		return "", "", fmt.Errorf("could not find vendor ID and/or model ID for interface %s", ifName)
	}

	return vendorID, modelID, nil
}
