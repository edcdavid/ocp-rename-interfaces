#!/bin/bash
# Example: Generate MachineConfig for a single interface with explicit naming

./bin/ocp-rename-interfaces \
  --macs "cc:aa:aa:aa:df:01" \
  --names "ptp0" \
  --mc-name "50-ptp-interface" \
  --output "single-interface.yaml"

echo "Generated single-interface.yaml"

