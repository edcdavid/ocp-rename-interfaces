# OpenShift Interface Renaming Tool

A Go-based CLI tool to generate and apply OpenShift MachineConfig resources for renaming network interfaces using systemd `.link` files. This tool is particularly useful for configuring PTP (Precision Time Protocol) interfaces in OpenShift clusters.

## Features

- 🎯 Generate MachineConfig YAML for network interface renaming
- 🔄 Support for both explicit interface naming and systemd NamePolicy
- 🎭 Automatic detection of single-node vs multi-node clusters
- ✅ Interactive confirmation before applying to cluster
- 🌐 Direct application to OpenShift clusters via kubeconfig
- 🏗️ Multi-platform builds (Linux, macOS, Windows)
- 📦 Clean, idiomatic Go code with comprehensive error handling

## Installation

### From Source

```bash
git clone https://github.com/deliedit/ocp-rename-interfaces.git
cd ocp-rename-interfaces
make install
```

### Download Binary

Download the latest release for your platform from the [releases page](https://github.com/deliedit/ocp-rename-interfaces/releases).

## Usage

### Generate MachineConfig with Explicit Interface Names

Rename interfaces to specific names (e.g., `ptp0`, `ptp1`). Names are matched to MACs in order:

```bash
ocp-rename-interfaces \
  --macs "cc:aa:aa:aa:df:01,cc:bb:bb:bb:df:02" \
  --names "ptp0,ptp1" \
  --output interface-config.yaml
```

This creates a MachineConfig that renames:
- Interface with MAC `cc:aa:aa:aa:df:01` → `ptp0` (first MAC → first name)
- Interface with MAC `cc:bb:bb:bb:df:02` → `ptp1` (second MAC → second name)

**Important:** The number of names must match the number of MAC addresses, and they are matched in order.

### Generate MachineConfig with NamePolicy

Use systemd's naming schemes instead of explicit names:

```bash
ocp-rename-interfaces \
  --macs "cc:aa:aa:aa:df:01" \
  --name-policies "slot,path,onboard" \
  --output interface-config.yaml
```

This creates systemd `.link` files that apply the `slot path onboard` NamePolicy to matched interfaces.

### Apply Directly to Cluster

Apply the MachineConfig directly to your OpenShift cluster:

```bash
ocp-rename-interfaces \
  --macs "cc:aa:aa:aa:df:01,cc:bb:bb:bb:df:02" \
  --names "ptp0,ptp1" \
  --kubeconfig ~/.kube/config \
  --apply
```

The tool will:
1. Detect if your cluster is single-node or multi-node
2. Display cluster information
3. Ask for confirmation before applying
4. Apply the MachineConfig with appropriate role label (`master` for single-node, `worker` for multi-node)

### Command-Line Options

| Flag | Short | Description | Required |
|------|-------|-------------|----------|
| `--macs` | `-m` | Comma-separated list of MAC addresses | Yes |
| `--names` | `-n` | Comma-separated list of interface names (must match number of MACs) | * |
| `--name-policies` | `-p` | Comma-separated list of NamePolicy schemes (e.g., slot,path,onboard) | * |
| `--kubeconfig` | `-k` | Path to kubeconfig file | No |
| `--output` | `-o` | Output file path (stdout if not specified) | No |
| `--apply` | `-a` | Apply MachineConfig to cluster | No |
| `--mc-name` | | MachineConfig resource name (default: 50-interface-rename) | No |

\* Either `--names` or `--name-policies` must be specified (mutually exclusive)

**Note:** When using `--names`, the number of names must exactly match the number of MACs. They are matched in order: the first name is assigned to the interface with the first MAC address, and so on.

## Examples

### Example 1: PTP Interface Configuration

Configure two interfaces for PTP:

```bash
ocp-rename-interfaces \
  --macs "00:1e:67:f1:23:45,00:1e:67:f1:23:46" \
  --names "ptp0,ptp1" \
  --mc-name 50-ptp-interfaces \
  --output ptp-config.yaml
```

### Example 2: Apply to Single-Node Cluster

```bash
ocp-rename-interfaces \
  --macs "00:1e:67:f1:23:45" \
  --names "ptp0" \
  --kubeconfig ~/sno-cluster/kubeconfig \
  --apply
```

Output:
```
Using kubeconfig: /home/user/sno-cluster/kubeconfig

Cluster information:
  Nodes: 1
  Control Plane Nodes: 1
  Worker Nodes: 0

⚠️  Single-node cluster detected - will use 'master' role label

Do you want to apply this MachineConfig to the cluster? (yes/no): yes
Created new MachineConfig: 50-interface-rename

✓ MachineConfig '50-interface-rename' applied successfully!

Note: The Machine Config Operator will roll out this change to the nodes.
This may take several minutes and will cause node reboots.
```

### Example 3: Using NamePolicy

Use systemd naming schemes:

```bash
ocp-rename-interfaces \
  --macs "00:1e:67:f1:23:45" \
  --name-policies "slot,path,onboard" \
  --output naming-policy.yaml
```

### Example 4: Custom Interface Names

Use any custom names in order:

```bash
ocp-rename-interfaces \
  --macs "aa:bb:cc:dd:ee:01,aa:bb:cc:dd:ee:02,aa:bb:cc:dd:ee:03" \
  --names "timing1,sync2,clock3" \
  --output custom-names.yaml
```

## Generated MachineConfig Structure

The tool generates MachineConfigs following this structure:

```yaml
apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfig
metadata:
  name: 50-interface-rename
  labels:
    machineconfiguration.openshift.io/role: worker  # or 'master' for single-node
spec:
  config:
    ignition:
      version: 3.2.0
    storage:
      files:
      - path: /etc/systemd/network/10-ptp0.link
        mode: 0644
        overwrite: true
        contents:
          source: data:text/plain,%5BMatch%5D%0AMACAddress%3D...
```

## Development

### Prerequisites

- Go 1.21 or later
- Make
- golangci-lint (for linting)

### Build

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Build for specific platform
make build-linux
make build-darwin
make build-windows
```

### Test

```bash
# Run tests
make test

# Run tests with coverage
make coverage
```

### Lint

```bash
# Run linters
make lint

# Format code
make fmt

# Run go vet
make vet
```

### Clean

```bash
make clean
```

## How It Works

1. **systemd .link Files**: The tool generates systemd network link files that are processed during early boot
2. **Ignition**: OpenShift uses Ignition to write these files to the node's filesystem
3. **MachineConfig Operator**: Monitors MachineConfig resources and applies them to nodes
4. **Node Reboot**: Changes require node reboot to take effect

### Interface Matching

Interfaces are matched by MAC address in the `[Match]` section of the `.link` file.

### Naming Methods

**Explicit Naming** (`--name`):
```
[Link]
Name=ptp0
```

**NamePolicy** (`--name-policy`):
```
[Link]
NamePolicy=slot path onboard
```

NamePolicy schemes (applied in order):
- `slot`: Uses PCI hotplug slot
- `path`: Uses physical location (PCI/USB path)
- `onboard`: Uses BIOS/firmware onboard index
- `mac`: Uses MAC address
- `keep`: Keeps existing name

## Troubleshooting

### Changes Not Applied

1. Check MachineConfig was created:
   ```bash
   oc get machineconfig 50-interface-rename
   ```

2. Check MachineConfig pool status:
   ```bash
   oc get mcp
   ```

3. Check node status:
   ```bash
   oc get nodes
   ```

### Interface Not Renamed

1. Verify MAC address is correct:
   ```bash
   ip link show
   ```

2. Check systemd link files on the node:
   ```bash
   oc debug node/<node-name>
   chroot /host
   ls -l /etc/systemd/network/
   cat /etc/systemd/network/10-ptp0.link
   ```

3. Check systemd logs:
   ```bash
   journalctl -u systemd-networkd
   ```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see LICENSE file for details

## Related Documentation

- [OpenShift MachineConfig](https://docs.openshift.com/container-platform/latest/post_installation_configuration/machine-configuration-tasks.html)
- [systemd.link](https://www.freedesktop.org/software/systemd/man/systemd.link.html)
- [Ignition Configuration](https://coreos.github.io/ignition/configuration-v3_2/)

