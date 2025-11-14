package machineconfig

import (
	"fmt"
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"
)

// MachineConfig represents an OpenShift MachineConfig resource
type MachineConfig struct {
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   Metadata          `yaml:"metadata"`
	Spec       MachineConfigSpec `yaml:"spec"`
}

type Metadata struct {
	Name   string            `yaml:"name"`
	Labels map[string]string `yaml:"labels"`
}

type MachineConfigSpec struct {
	Config Config `yaml:"config"`
}

type Config struct {
	Ignition Ignition `yaml:"ignition"`
	Storage  Storage  `yaml:"storage"`
}

type Ignition struct {
	Version string `yaml:"version"`
}

type Storage struct {
	Files []File `yaml:"files"`
}

type File struct {
	Path      string   `yaml:"path"`
	Mode      int      `yaml:"mode"`
	Overwrite bool     `yaml:"overwrite"`
	Contents  Contents `yaml:"contents"`
	Comment   string   `yaml:"-"` // Not serialized to YAML, only used for generating comments
}

type Contents struct {
	Source string `yaml:"source"`
}

const (
	// DefaultFileMode is the default permission mode for systemd .link files
	DefaultFileMode = 0o644
	// DefaultConfigFileMode is the default permission mode for output config files
	DefaultConfigFileMode = 0o600
)

// NewMachineConfigWithNames creates a MachineConfig with explicit interface names using a prefix
//
// Deprecated: Use NewMachineConfigWithExplicitNames for more control
func NewMachineConfigWithNames(name, role string, macAddresses []string, namePrefix string) (*MachineConfig, error) {
	files := make([]File, 0, len(macAddresses))

	for i, mac := range macAddresses {
		interfaceName := fmt.Sprintf("%s%d", namePrefix, i)
		linkFile := generateLinkFileWithName(mac, interfaceName)
		encodedContent := encodeLinkFile(linkFile)

		files = append(files, File{
			Path:      fmt.Sprintf("/etc/systemd/network/10-%s.link", interfaceName),
			Mode:      DefaultFileMode,
			Overwrite: true,
			Contents: Contents{
				Source: encodedContent,
			},
		})
	}

	return createMachineConfig(name, role, files), nil
}

// NewMachineConfigWithExplicitNames creates a MachineConfig with explicit interface names
// The names slice must have the same length as macAddresses, and they are matched in order:
// names[0] will be assigned to the interface with macAddresses[0], etc.
func NewMachineConfigWithExplicitNames(name, role string, macAddresses, names []string) (*MachineConfig, error) {
	if len(macAddresses) != len(names) {
		return nil, fmt.Errorf("number of MAC addresses (%d) must match number of names (%d)", len(macAddresses), len(names))
	}

	files := make([]File, 0, len(macAddresses))

	for i, mac := range macAddresses {
		interfaceName := names[i]
		linkFile := generateLinkFileWithName(mac, interfaceName)
		encodedContent := encodeLinkFile(linkFile)

		files = append(files, File{
			Path:      fmt.Sprintf("/etc/systemd/network/10-%s.link", interfaceName),
			Mode:      DefaultFileMode,
			Overwrite: true,
			Contents: Contents{
				Source: encodedContent,
			},
			Comment: linkFile, // Store decoded content for YAML comment
		})
	}

	return createMachineConfig(name, role, files), nil
}

// NewMachineConfigWithPolicy creates a MachineConfig with NamePolicy
func NewMachineConfigWithPolicy(name, role string, macAddresses []string, namePolicy string) (*MachineConfig, error) {
	files := make([]File, 0, len(macAddresses))

	for _, mac := range macAddresses {
		linkFile := generateLinkFileWithPolicy(mac, namePolicy)
		encodedContent := encodeLinkFile(linkFile)

		// Use MAC address in filename to ensure uniqueness
		safeMac := strings.ReplaceAll(mac, ":", "")
		files = append(files, File{
			Path:      fmt.Sprintf("/etc/systemd/network/10-interface-%s.link", safeMac),
			Mode:      DefaultFileMode,
			Overwrite: true,
			Contents: Contents{
				Source: encodedContent,
			},
			Comment: linkFile, // Store decoded content for YAML comment
		})
	}

	return createMachineConfig(name, role, files), nil
}

// NewMachineConfigWithPropertyAndName creates a MachineConfig with Property-based matching and explicit name
func NewMachineConfigWithPropertyAndName(name, role, vendorID, modelID, interfaceName string) (*MachineConfig, error) {
	linkFile := generateLinkFileWithPropertyAndName(vendorID, modelID, interfaceName)
	encodedContent := encodeLinkFile(linkFile)

	files := []File{
		{
			Path:      fmt.Sprintf("/etc/systemd/network/10-%s.link", interfaceName),
			Mode:      DefaultFileMode,
			Overwrite: true,
			Contents: Contents{
				Source: encodedContent,
			},
			Comment: linkFile,
		},
	}

	return createMachineConfig(name, role, files), nil
}

// NewMachineConfigWithPropertyAndPolicy creates a MachineConfig with Property-based matching and NamePolicy
func NewMachineConfigWithPropertyAndPolicy(name, role, vendorID, modelID, namePolicy string) (*MachineConfig, error) {
	linkFile := generateLinkFileWithPropertyAndPolicy(vendorID, modelID, namePolicy)
	encodedContent := encodeLinkFile(linkFile)

	// Create a safe filename using vendor and model IDs
	safeVendor := strings.ReplaceAll(vendorID, "0x", "")
	safeModel := strings.ReplaceAll(modelID, "0x", "")
	filename := fmt.Sprintf("10-interface-%s-%s.link", safeVendor, safeModel)

	files := []File{
		{
			Path:      fmt.Sprintf("/etc/systemd/network/%s", filename),
			Mode:      DefaultFileMode,
			Overwrite: true,
			Contents: Contents{
				Source: encodedContent,
			},
			Comment: linkFile,
		},
	}

	return createMachineConfig(name, role, files), nil
}

func createMachineConfig(name, role string, files []File) *MachineConfig {
	return &MachineConfig{
		APIVersion: "machineconfiguration.openshift.io/v1",
		Kind:       "MachineConfig",
		Metadata: Metadata{
			Name: name,
			Labels: map[string]string{
				"machineconfiguration.openshift.io/role": role,
			},
		},
		Spec: MachineConfigSpec{
			Config: Config{
				Ignition: Ignition{
					Version: "3.2.0",
				},
				Storage: Storage{
					Files: files,
				},
			},
		},
	}
}

func generateLinkFileWithName(macAddress, interfaceName string) string {
	return fmt.Sprintf(`[Match]
MACAddress=%s

[Link]
Name=%s
`, macAddress, interfaceName)
}

func generateLinkFileWithPolicy(macAddress, namePolicy string) string {
	return fmt.Sprintf(`[Match]
MACAddress=%s

[Link]
NamePolicy=%s
`, macAddress, namePolicy)
}

func generateLinkFileWithPropertyAndName(vendorID, modelID, interfaceName string) string {
	// Ensure 0x prefix - udev properties include the 0x prefix
	if !strings.HasPrefix(vendorID, "0x") {
		vendorID = "0x" + vendorID
	}
	if !strings.HasPrefix(modelID, "0x") {
		modelID = "0x" + modelID
	}

	return fmt.Sprintf(`[Match]
Property=ID_VENDOR_ID=%s
Property=ID_MODEL_ID=%s

[Link]
Name=%s
`, vendorID, modelID, interfaceName)
}

func generateLinkFileWithPropertyAndPolicy(vendorID, modelID, namePolicy string) string {
	// Ensure 0x prefix - udev properties include the 0x prefix
	if !strings.HasPrefix(vendorID, "0x") {
		vendorID = "0x" + vendorID
	}
	if !strings.HasPrefix(modelID, "0x") {
		modelID = "0x" + modelID
	}

	return fmt.Sprintf(`[Match]
Property=ID_VENDOR_ID=%s
Property=ID_MODEL_ID=%s

[Link]
NamePolicy=%s
`, vendorID, modelID, namePolicy)
}

func encodeLinkFile(content string) string {
	// URL encode the content
	encoded := url.QueryEscape(content)
	// Replace + with %20 for proper space encoding
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	return "data:text/plain," + encoded
}

// MarshalMachineConfig converts a MachineConfig to YAML with comments showing decoded content
func MarshalMachineConfig(mc *MachineConfig) ([]byte, error) {
	data, err := yaml.Marshal(mc)
	if err != nil {
		return nil, err
	}

	// Add comments with decoded link file content
	result := addLinkFileComments(string(data), mc)
	return []byte(result), nil
}

func addLinkFileComments(yamlContent string, mc *MachineConfig) string {
	lines := strings.Split(yamlContent, "\n")
	var result strings.Builder

	fileIndex := 0

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		result.WriteString(line)
		result.WriteString("\n")

		// Detect when we're entering a file entry
		if strings.Contains(line, "- path: /etc/systemd/network/") && fileIndex < len(mc.Spec.Config.Storage.Files) {
			file := mc.Spec.Config.Storage.Files[fileIndex]

			// Add comment with decoded content after the path line
			if file.Comment != "" {
				// Add indented comment block
				commentLines := strings.Split(file.Comment, "\n")
				for _, commentLine := range commentLines {
					if commentLine != "" {
						result.WriteString("          # ")
						result.WriteString(commentLine)
						result.WriteString("\n")
					}
				}
			}

			fileIndex++
		}
	}

	return result.String()
}
