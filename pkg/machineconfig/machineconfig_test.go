package machineconfig

import (
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNewMachineConfigWithNames(t *testing.T) {
	tests := []struct {
		name          string
		mcName        string
		role          string
		macAddresses  []string
		namePrefix    string
		expectedFiles int
	}{
		{
			name:          "Single interface",
			mcName:        "test-mc",
			role:          "worker",
			macAddresses:  []string{"aa:bb:cc:dd:ee:ff"},
			namePrefix:    "ptp",
			expectedFiles: 1,
		},
		{
			name:          "Multiple interfaces",
			mcName:        "test-mc",
			role:          "master",
			macAddresses:  []string{"aa:bb:cc:dd:ee:ff", "11:22:33:44:55:66"},
			namePrefix:    "ptp",
			expectedFiles: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc, err := NewMachineConfigWithNames(tt.mcName, tt.role, tt.macAddresses, tt.namePrefix)
			if err != nil {
				t.Fatalf("NewMachineConfigWithNames() error = %v", err)
			}

			if mc.Metadata.Name != tt.mcName {
				t.Errorf("Expected name %s, got %s", tt.mcName, mc.Metadata.Name)
			}

			if mc.Metadata.Labels["machineconfiguration.openshift.io/role"] != tt.role {
				t.Errorf("Expected role %s, got %s", tt.role, mc.Metadata.Labels["machineconfiguration.openshift.io/role"])
			}

			if len(mc.Spec.Config.Storage.Files) != tt.expectedFiles {
				t.Errorf("Expected %d files, got %d", tt.expectedFiles, len(mc.Spec.Config.Storage.Files))
			}

			// Verify file names are correct
			for i, file := range mc.Spec.Config.Storage.Files {
				expectedPath := fmt.Sprintf("/etc/systemd/network/10-%s%d.link", tt.namePrefix, i)
				if file.Path != expectedPath {
					t.Errorf("Expected path %s, got %s", expectedPath, file.Path)
				}
			}
		})
	}
}

func TestNewMachineConfigWithExplicitNames(t *testing.T) {
	tests := []struct {
		name          string
		mcName        string
		role          string
		macAddresses  []string
		names         []string
		expectedFiles int
		expectError   bool
	}{
		{
			name:          "Single interface",
			mcName:        "test-mc",
			role:          "worker",
			macAddresses:  []string{"aa:bb:cc:dd:ee:ff"},
			names:         []string{"ptp0"},
			expectedFiles: 1,
			expectError:   false,
		},
		{
			name:          "Multiple interfaces in order",
			mcName:        "test-mc",
			role:          "master",
			macAddresses:  []string{"aa:bb:cc:dd:ee:ff", "11:22:33:44:55:66", "33:44:55:66:77:88"},
			names:         []string{"ptp0", "ptp1", "ptp2"},
			expectedFiles: 3,
			expectError:   false,
		},
		{
			name:          "Custom names",
			mcName:        "test-mc",
			role:          "worker",
			macAddresses:  []string{"aa:bb:cc:dd:ee:ff", "11:22:33:44:55:66"},
			names:         []string{"timing1", "timing2"},
			expectedFiles: 2,
			expectError:   false,
		},
		{
			name:         "Mismatched count - too few names",
			mcName:       "test-mc",
			role:         "worker",
			macAddresses: []string{"aa:bb:cc:dd:ee:ff", "11:22:33:44:55:66"},
			names:        []string{"ptp0"},
			expectError:  true,
		},
		{
			name:         "Mismatched count - too many names",
			mcName:       "test-mc",
			role:         "worker",
			macAddresses: []string{"aa:bb:cc:dd:ee:ff"},
			names:        []string{"ptp0", "ptp1"},
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc, err := NewMachineConfigWithExplicitNames(tt.mcName, tt.role, tt.macAddresses, tt.names)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("NewMachineConfigWithExplicitNames() error = %v", err)
			}

			if mc.Metadata.Name != tt.mcName {
				t.Errorf("Expected name %s, got %s", tt.mcName, mc.Metadata.Name)
			}

			if mc.Metadata.Labels["machineconfiguration.openshift.io/role"] != tt.role {
				t.Errorf("Expected role %s, got %s", tt.role, mc.Metadata.Labels["machineconfiguration.openshift.io/role"])
			}

			if len(mc.Spec.Config.Storage.Files) != tt.expectedFiles {
				t.Errorf("Expected %d files, got %d", tt.expectedFiles, len(mc.Spec.Config.Storage.Files))
			}

			// Verify file names match the provided names in order
			for i, file := range mc.Spec.Config.Storage.Files {
				expectedPath := fmt.Sprintf("/etc/systemd/network/10-%s.link", tt.names[i])
				if file.Path != expectedPath {
					t.Errorf("Expected path %s, got %s", expectedPath, file.Path)
				}

				// Verify the content contains the correct MAC and name
				if !strings.Contains(file.Contents.Source, tt.names[i]) {
					t.Errorf("Expected source to contain name %s", tt.names[i])
				}
			}
		})
	}
}

func TestNewMachineConfigWithPolicy(t *testing.T) {
	tests := []struct {
		name          string
		mcName        string
		role          string
		macAddresses  []string
		namePolicy    string
		expectedFiles int
	}{
		{
			name:          "Single policy",
			mcName:        "test-mc",
			role:          "worker",
			macAddresses:  []string{"aa:bb:cc:dd:ee:ff"},
			namePolicy:    "slot",
			expectedFiles: 1,
		},
		{
			name:          "Path policy",
			mcName:        "test-mc",
			role:          "worker",
			macAddresses:  []string{"aa:bb:cc:dd:ee:ff"},
			namePolicy:    "path",
			expectedFiles: 1,
		},
		{
			name:          "Multiple interfaces same policy",
			mcName:        "test-mc",
			role:          "master",
			macAddresses:  []string{"aa:bb:cc:dd:ee:ff", "11:22:33:44:55:66"},
			namePolicy:    "slot",
			expectedFiles: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc, err := NewMachineConfigWithPolicy(tt.mcName, tt.role, tt.macAddresses, tt.namePolicy)
			if err != nil {
				t.Fatalf("NewMachineConfigWithPolicy() error = %v", err)
			}

			if len(mc.Spec.Config.Storage.Files) != tt.expectedFiles {
				t.Errorf("Expected %d files, got %d", tt.expectedFiles, len(mc.Spec.Config.Storage.Files))
			}

			// Verify the source contains the policy
			for _, file := range mc.Spec.Config.Storage.Files {
				if !strings.Contains(file.Contents.Source, "NamePolicy") {
					t.Errorf("Expected NamePolicy in source, got %s", file.Contents.Source)
				}
				if !strings.Contains(file.Contents.Source, tt.namePolicy) {
					t.Errorf("Expected policy %s in source", tt.namePolicy)
				}
			}
		})
	}
}

func TestGenerateLinkFileWithName(t *testing.T) {
	mac := "aa:bb:cc:dd:ee:ff"
	name := "ptp0"

	result := generateLinkFileWithName(mac, name)

	if !strings.Contains(result, "[Match]") {
		t.Error("Expected [Match] section")
	}
	if !strings.Contains(result, "MACAddress="+mac) {
		t.Error("Expected MAC address in result")
	}
	if !strings.Contains(result, "[Link]") {
		t.Error("Expected [Link] section")
	}
	if !strings.Contains(result, "Name="+name) {
		t.Error("Expected interface name in result")
	}
}

func TestGenerateLinkFileWithPolicy(t *testing.T) {
	mac := "aa:bb:cc:dd:ee:ff"
	policy := "slot"

	result := generateLinkFileWithPolicy(mac, policy)

	if !strings.Contains(result, "[Match]") {
		t.Error("Expected [Match] section")
	}
	if !strings.Contains(result, "MACAddress="+mac) {
		t.Error("Expected MAC address in result")
	}
	if !strings.Contains(result, "[Link]") {
		t.Error("Expected [Link] section")
	}
	if !strings.Contains(result, "NamePolicy=slot") {
		t.Error("Expected NamePolicy in result")
	}
}

func TestEncodeLinkFile(t *testing.T) {
	content := `[Match]
MACAddress=aa:bb:cc:dd:ee:ff

[Link]
Name=ptp0
`

	encoded := encodeLinkFile(content)

	if !strings.HasPrefix(encoded, "data:text/plain,") {
		t.Error("Expected data:text/plain, prefix")
	}

	// Check that special characters are properly encoded
	if !strings.Contains(encoded, "%5BMatch%5D") { // [Match]
		t.Error("Expected URL-encoded [Match]")
	}
	if !strings.Contains(encoded, "%0A") { // newline
		t.Error("Expected URL-encoded newlines")
	}
}

func TestMarshalMachineConfig(t *testing.T) {
	mc, err := NewMachineConfigWithNames("test-mc", "worker", []string{"aa:bb:cc:dd:ee:ff"}, "ptp")
	if err != nil {
		t.Fatalf("Failed to create MachineConfig: %v", err)
	}

	yamlData, err := MarshalMachineConfig(mc)
	if err != nil {
		t.Fatalf("MarshalMachineConfig() error = %v", err)
	}

	// Unmarshal to verify it's valid YAML
	var result map[string]interface{}
	if err := yaml.Unmarshal(yamlData, &result); err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify basic structure
	if result["apiVersion"] != "machineconfiguration.openshift.io/v1" {
		t.Error("Invalid apiVersion")
	}
	if result["kind"] != "MachineConfig" {
		t.Error("Invalid kind")
	}
}
