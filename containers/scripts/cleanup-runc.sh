#!/bin/bash

# ==============================================================================
# Cleanup script for runc container
# ==============================================================================

export CONTAINER_ID=container-1

echo "Cleaning up container: ${CONTAINER_ID}"

# Kill container
echo "Killing container..."
sudo runc kill ${CONTAINER_ID} TERM || echo "Container already stopped"

# Wait a moment for it to stop
sleep 2

# Delete container
echo "Deleting container..."
sudo runc delete ${CONTAINER_ID} || echo "Container already deleted"

# Clean up networking
echo "Cleaning up networking..."
sudo ip link delete veth0 2>/dev/null || echo "veth0 already deleted"
sudo rm -f /run/netns/${CONTAINER_ID} || echo "Network namespace link already removed"

# Remove bundle directory
echo "Removing bundle directory..."
rm -rf ~/${CONTAINER_ID}

echo "Cleanup complete."
