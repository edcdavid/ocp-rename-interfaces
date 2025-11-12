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
	// Parse comma-separated MAC addresses
	macs := parseMACAddresses(macAddresses)
	if len(macs) == 0 {
		return fmt.Errorf("at least one MAC address must be specified")
	}

	// Parse naming options
	var policies []string
	var names []string

	if namePolicies != "" {
		policies = parseCommaSeparated(namePolicies)
	}
	if interfaceNames != "" {
		names = parseCommaSeparated(interfaceNames)
	}

	// Validate inputs
	if len(policies) == 0 && len(names) == 0 {
		return fmt.Errorf("either --name-policies or --names must be specified")
	}

	if len(policies) > 0 && len(names) > 0 {
		return fmt.Errorf("--name-policies and --names are mutually exclusive")
	}

	// If using --names, count must match MACs count
	if len(names) > 0 && len(names) != len(macs) {
		return fmt.Errorf("number of names (%d) must match number of MAC addresses (%d)", len(names), len(macs))
	}

	// Generate the MachineConfig
	var mc *machineconfig.MachineConfig
	var err error

	if apply {
		// Determine if cluster is single-node
		kubeconfigPath := getKubeconfigPath()

		fmt.Printf("Using kubeconfig: %s\n", kubeconfigPath)
		fmt.Print("\nCluster information:\n")

		isSingleNode, clusterInfo, err := machineconfig.IsClusterSingleNode(kubeconfigPath)
		if err != nil {
			return fmt.Errorf("failed to detect cluster topology: %w", err)
		}

		fmt.Println(clusterInfo)

		if isSingleNode {
			fmt.Println("\n⚠️  Single-node cluster detected - will use 'master' role label")
		} else {
			fmt.Println("\n✓ Multi-node cluster detected - will use 'worker' role label")
		}

		// Ask for confirmation
		fmt.Print("\nDo you want to apply this MachineConfig to the cluster? (yes/no): ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "yes" && response != "y" {
			fmt.Println("Aborted.")
			return nil
		}

		// Generate with appropriate role
		mc, err = generateMachineConfig(isSingleNode, macs, names, policies)
		if err != nil {
			return err
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

	// Generate without applying
	mc, err = generateMachineConfig(false, macs, names, policies) // default to worker for file generation
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
		if err := os.WriteFile(output, yamlData, 0600); err != nil {
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
