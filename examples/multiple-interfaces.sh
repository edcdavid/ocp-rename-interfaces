#!/bin/bash
# Example: Generate MachineConfig for multiple interfaces with explicit naming
# The names are matched to MACs in order: ptp0→first MAC, ptp1→second MAC, etc.

./bin/ocp-rename-interfaces \
  --macs "cc:aa:aa:aa:df:01,cc:bb:bb:bb:df:02,cc:cc:cc:cc:df:03" \
  --names "ptp0,ptp1,ptp2" \
  --mc-name "50-ptp-interfaces" \
  --output "multiple-interfaces.yaml"

echo "Generated multiple-interfaces.yaml"

