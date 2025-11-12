package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/deliedit/ocp-rename-interfaces/pkg/machineconfig"
	"github.com/spf13/cobra"
)

var (
	macAddresses   string
	namePolicies   string
	interfaceNames string
	kubeconfig     string
	output         string
	apply          bool
	mcName         string
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
	rootCmd.Flags().StringVarP(&namePolicies, "name-policies", "p", "", "Comma-separated list of NamePolicy schemes (e.g., slot,path,onboard)")
	rootCmd.Flags().StringVarP(&interfaceNames, "names", "n", "", "Comma-separated list of interface names (e.g., ptp0,ptp1). Must match number of MACs.")
	rootCmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "k", "", "Path to kubeconfig file (uses KUBECONFIG env or ~/.kube/config if not specified)")
	rootCmd.Flags().StringVarP(&output, "output", "o", "", "Output file path (prints to stdout if not specified)")
	rootCmd.Flags().BoolVarP(&apply, "apply", "a", false, "Apply the MachineConfig to the cluster")
	rootCmd.Flags().StringVar(&mcName, "mc-name", "50-interface-rename", "Name of the MachineConfig resource")

	// Mark required flags - error is handled by cobra during execution
	_ = rootCmd.MarkFlagRequired("macs")
}

func Execute() error {
	return rootCmd.Execute()
}

func runRoot(cmd *cobra.Command, args []string) error {
	macs, names, policies, err := parseAndValidateFlags()
	if err != nil {
		return err
	}

	if apply {
		return applyToCluster(macs, names, policies)
	}

	return generateAndOutput(macs, names, policies)
}

func parseAndValidateFlags() (macs, names, policies []string, err error) {
	// Parse comma-separated MAC addresses
	macs = parseMACAddresses(macAddresses)
	if len(macs) == 0 {
		return nil, nil, nil, fmt.Errorf("at least one MAC address must be specified")
	}

	// Parse naming options
	if namePolicies != "" {
		policies = parseCommaSeparated(namePolicies)
	}
	if interfaceNames != "" {
		names = parseCommaSeparated(interfaceNames)
	}

	// Validate inputs
	if len(policies) == 0 && len(names) == 0 {
		return nil, nil, nil, fmt.Errorf("either --name-policies or --names must be specified")
	}

	if len(policies) > 0 && len(names) > 0 {
		return nil, nil, nil, fmt.Errorf("--name-policies and --names are mutually exclusive")
	}

	// If using --names, count must match MACs count
	if len(names) > 0 && len(names) != len(macs) {
		return nil, nil, nil, fmt.Errorf("number of names (%d) must match number of MAC addresses (%d)", len(names), len(macs))
	}

	return macs, names, policies, nil
}

func applyToCluster(macs, names, policies []string) error {
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
	mc, err := generateMachineConfig(isSingleNode, macs, names, policies)
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

func generateAndOutput(macs, names, policies []string) error {
	// Generate without applying (default to worker for file generation)
	mc, err := generateMachineConfig(false, macs, names, policies)
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

func generateMachineConfig(isSingleNode bool, macs, names, policies []string) (*machineconfig.MachineConfig, error) {
	role := "worker"
	if isSingleNode {
		role = "master"
	}

	if len(names) > 0 {
		return machineconfig.NewMachineConfigWithExplicitNames(mcName, role, macs, names)
	}

	return machineconfig.NewMachineConfigWithPolicy(mcName, role, macs, policies)
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
