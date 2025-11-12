#!/bin/bash
# Example: Generate MachineConfig using NamePolicy instead of explicit names

./bin/ocp-rename-interfaces \
  --macs "cc:aa:aa:aa:df:01" \
  --name-policies "slot,path,onboard" \
  --mc-name "50-interface-namepolicy" \
  --output "name-policy.yaml"

echo "Generated name-policy.yaml"

