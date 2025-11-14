#!/bin/bash
# Example: Generate MachineConfig using vendor/model ID matching

echo "Example 1: Manual vendor/model IDs with explicit interface name"
./bin/ocp-rename-interfaces \
  --vendor "0x8086" \
  --model "0x153a" \
  --names "ptp0" \
  --mc-name "50-intel-i211-interface" \
  --output "vendor-model-explicit-name.yaml"

echo ""
echo "Example 2: Manual vendor/model IDs with NamePolicy"
./bin/ocp-rename-interfaces \
  --vendor "0x8086" \
  --model "0x153a" \
  --name-policy "slot" \
  --mc-name "50-intel-i211-policy" \
  --output "vendor-model-name-policy.yaml"

echo ""
echo "Example 3: Auto-detect vendor/model from local interface (Linux only)"
echo "Note: This requires udevadm and a real network interface on the local machine"
echo "# ./bin/ocp-rename-interfaces \\"
echo "#   --refIfName \"enp0s3\" \\"
echo "#   --names \"ptp0\" \\"
echo "#   --mc-name \"50-auto-detected-interface\" \\"
echo "#   --output \"auto-detected-local.yaml\""

echo ""
echo "Example 4: Auto-detect vendor/model from cluster node"
echo "Note: This requires oc CLI, kubeconfig, and access to the cluster"
echo "# ./bin/ocp-rename-interfaces \\"
echo "#   --refIfName \"eno1\" \\"
echo "#   --node \"worker-0\" \\"
echo "#   --kubeconfig ~/.kube/config \\"
echo "#   --names \"ptp0\" \\"
echo "#   --mc-name \"50-auto-detected-cluster\" \\"
echo "#   --output \"auto-detected-cluster.yaml\""

echo ""
echo "Generated vendor-model-explicit-name.yaml and vendor-model-name-policy.yaml"

