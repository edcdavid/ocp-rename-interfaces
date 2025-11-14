# OpenShift Interface Renaming Tool

A Go-based CLI tool to generate and apply OpenShift MachineConfig resources for renaming network interfaces using systemd `.link` files. This tool is particularly useful for configuring PTP (Precision Time Protocol) interfaces in OpenShift clusters.

## Features

- 🎯 Generate MachineConfig YAML for network interface renaming
- 🔄 Support for both explicit interface naming and systemd NamePolicy
- 🔌 Match interfaces by MAC address or vendor/model ID
- 🔍 Auto-detect vendor/model IDs from existing interfaces using udevadm
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
  --name-policy "slot" \
  --output interface-config.yaml
```

This creates systemd `.link` files that apply the `slot` NamePolicy to matched interfaces.

### Match by Vendor/Model ID (Manual)

Match interfaces based on their PCI vendor and model IDs instead of MAC addresses:

```bash
ocp-rename-interfaces \
  --vendor "0x8086" \
  --model "0x153a" \
  --names "ptp0" \
  --output interface-config.yaml
```

This matches any Intel (0x8086) I211 (0x153a) network card and renames it to `ptp0`. The MachineConfig will be named `50-interface-8086-153a` (automatically includes vendor/model IDs).

### Auto-detect Vendor/Model ID (Local Machine)

Automatically detect vendor and model IDs from a local interface (requires `udevadm` on Linux):

```bash
ocp-rename-interfaces \
  --refIfName "enp0s3" \
  --names "ptp0" \
  --output interface-config.yaml
```

This runs `udevadm info -q property -p /sys/class/net/enp0s3` to extract the vendor and model IDs.

### Auto-detect from Cluster Node

Automatically detect vendor and model IDs from an interface on a cluster node (requires `oc` CLI and cluster access):

```bash
ocp-rename-interfaces \
  --refIfName "eno1" \
  --node "worker-0" \
  --kubeconfig ~/.kube/config \
  --names "ptp0" \
  --output interface-config.yaml
```

This runs `oc debug node/worker-0 -- chroot /host udevadm info -q property -p /sys/class/net/eno1` to extract the vendor and model IDs from the cluster node, then generates a MachineConfig that will match any interface with the same hardware.

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
| `--macs` | `-m` | Comma-separated list of MAC addresses | ** |
| `--vendor` | | Vendor ID in hex format (e.g., 0x8086) | ** |
| `--model` | | Model ID in hex format (e.g., 0x153a) | ** |
| `--refIfName` | | Reference interface to auto-detect vendor/model IDs | ** |
| `--node` | | Node name for remote detection (use with --refIfName) | No |
| `--names` | `-n` | Comma-separated list of interface names (must match number of MACs) | * |
| `--name-policy` | `-p` | NamePolicy scheme (e.g., slot, path, onboard, mac, keep) | * |
| `--kubeconfig` | `-k` | Path to kubeconfig file | No |
| `--output` | `-o` | Output file path (stdout if not specified) | No |
| `--apply` | `-a` | Apply MachineConfig to cluster | No |
| `--mc-name` | | MachineConfig resource name (default: 50-interface-rename, or 50-interface-VENDOR-MODEL when using vendor/model) | No |

\* Either `--names` or `--name-policy` must be specified (mutually exclusive)

\*\* At least one matching method must be specified:
  - `--macs` for MAC address matching
  - `--vendor` and `--model` together for property-based matching
  - `--refIfName` for auto-detection of vendor/model IDs (cannot be combined with `--vendor`/`--model`)

**Matching Notes:**
- When using `--macs` with `--names`, the number of names must match the number of MACs (matched in order)
- When using `--vendor`/`--model`, only one interface name can be specified (all matching interfaces get the same name)
- `--refIfName` cannot be combined with manual `--vendor`/`--model`
- `--refIfName` without `--node`: Detects from local machine (requires `udevadm` on Linux)
- `--refIfName` with `--node`: Detects from cluster node (requires `oc` CLI and `--kubeconfig`)
- `--node` requires `--refIfName` and `--kubeconfig`

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

### Example 5: Vendor/Model ID Matching

Match any Intel I211 network card:

```bash
ocp-rename-interfaces \
  --vendor "0x8086" \
  --model "0x153a" \
  --names "ptp0" \
  --mc-name "50-intel-i211-interface" \
  --output intel-i211.yaml
```

### Example 6: Auto-detect from Local Interface

Auto-detect vendor/model from a local interface (Linux only):

```bash
ocp-rename-interfaces \
  --refIfName "enp0s3" \
  --names "ptp0" \
  --output auto-detected.yaml
```

This will run `udevadm info -q property -p /sys/class/net/enp0s3` locally.

### Example 7: Auto-detect from Cluster Node

Auto-detect vendor/model from an interface on a cluster node:

```bash
ocp-rename-interfaces \
  --refIfName "eno1" \
  --node "worker-0" \
  --kubeconfig ~/.kube/config \
  --names "ptp0" \
  --output auto-detected-cluster.yaml
```

This will run `oc debug node/worker-0 -- chroot /host udevadm info -q property -p /sys/class/net/eno1` to detect the vendor and model IDs from the cluster node.

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

Interfaces can be matched in two ways in the `[Match]` section of the `.link` file:

**MAC Address Matching:**
```
[Match]
MACAddress=aa:bb:cc:dd:ee:ff
```

**Property-based Matching (Vendor/Model ID):**
```
[Match]
Property=ID_VENDOR_ID=0x8086
Property=ID_MODEL_ID=0x153a
```

Property-based matching is useful when you want to match any interface of a specific hardware type, regardless of its MAC address.

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

## How to Find Vendor/Model IDs

### Using udevadm (Local Linux)

```bash
udevadm info -q property -p /sys/class/net/enp0s3 | grep -E "ID_VENDOR_ID|ID_MODEL_ID"
```

Example output:
```
ID_VENDOR_ID=8086
ID_MODEL_ID=153a
```

Note: The tool uses `Property=ID_VENDOR_ID` and `Property=ID_MODEL_ID` to match against udev properties in `.link` files. The values must include the `0x` prefix to match what udev reports.

### Using oc debug (Cluster Node)

```bash
oc debug node/worker-0 -- chroot /host udevadm info -q property -p /sys/class/net/eno1 | grep -E "ID_VENDOR_ID|ID_MODEL_ID"
```

### Using lspci

```bash
lspci -nn | grep Ethernet
```

Example output:
```
00:03.0 Ethernet controller [0200]: Intel Corporation I211 Gigabit Network Connection [8086:153a] (rev 03)
```

The vendor ID is `8086` and model ID is `153a`. Add `0x` prefix when using with the tool.

### Common Vendor/Model IDs

**Intel Network Cards:**
- Intel I210: `--vendor 0x8086 --model 0x1533`
- Intel I211: `--vendor 0x8086 --model 0x153a`
- Intel I350: `--vendor 0x8086 --model 0x1521`
- Intel 82599: `--vendor 0x8086 --model 0x10fb`

**Broadcom Network Cards:**
- BCM5720: `--vendor 0x14e4 --model 0x165f`
- BCM57810: `--vendor 0x14e4 --model 0x168e`

**Mellanox Network Cards:**
- ConnectX-4: `--vendor 0x15b3 --model 0x1013`
- ConnectX-5: `--vendor 0x15b3 --model 0x1017`

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

### Vendor/Model Detection Issues

**Error: "could not find vendor ID and/or model ID"**

Possible causes:
- Interface doesn't exist
- Interface is virtual (no PCI vendor/model IDs)
- Wrong interface name

Solutions:
- Verify interface exists: `ip link show` or `oc debug node/<node> -- chroot /host ip link`
- Try a different physical interface
- Check interface name spelling

**Error: "failed to execute oc debug node"**

Possible causes:
- `oc` CLI not installed
- Invalid kubeconfig
- No cluster access
- Insufficient permissions

Solutions:
- Install `oc` CLI: Check [OpenShift CLI installation](https://docs.openshift.com/container-platform/latest/cli_reference/openshift_cli/getting-started-cli.html)
- Verify cluster access: `oc get nodes`
- Check kubeconfig: `echo $KUBECONFIG` or use `--kubeconfig` flag
- Ensure you have permissions to create debug pods

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see LICENSE file for details

## Related Documentation

- [OpenShift MachineConfig](https://docs.openshift.com/container-platform/latest/post_installation_configuration/machine-configuration-tasks.html)
- [systemd.link](https://www.freedesktop.org/software/systemd/man/systemd.link.html)
- [Ignition Configuration](https://coreos.github.io/ignition/configuration-v3_2/)

