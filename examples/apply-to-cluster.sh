#!/bin/bash
# Example: Apply MachineConfig directly to cluster
# This will prompt for confirmation before applying

./bin/ocp-rename-interfaces \
  --macs "cc:aa:aa:aa:df:01,cc:bb:bb:bb:df:02" \
  --names "ptp0,ptp1" \
  --mc-name "50-ptp-interfaces" \
  --kubeconfig "${HOME}/.kube/config" \
  --apply

echo "MachineConfig applied to cluster"

