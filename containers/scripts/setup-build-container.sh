#!/bin/bash

# ==============================================================================
# Script to setup and run a container using runc
# This script is intended to be run inside a Linux VM
# ==============================================================================

set -e

echo "Installing dependencies..."
sudo apt update
sudo apt install -y runc docker.io docker-compose docker-cli jq bridge-utils net-tools

# Add user to docker group
sudo usermod -aG docker $USER

# Setup the Container
export CONTAINER_ID=container-1
echo "Setting up container: ${CONTAINER_ID}"

mkdir -p ~/${CONTAINER_ID}
cd ~/${CONTAINER_ID}

echo "Generating OCI spec..."
runc spec

mkdir -p rootfs

# Pull and export nginx image
export IMAGE=ghcr.io/iximiuz/labs/nginx:alpine
echo "Pulling and exporting image: ${IMAGE}"
sudo docker pull $IMAGE
sudo docker export $(sudo docker create $IMAGE) | tar -C rootfs -xf -

# Create required directories and set permissions for nginx
echo "Preparing rootfs for nginx..."
mkdir -p rootfs/var/cache/nginx/client_temp
mkdir -p rootfs/var/cache/nginx/proxy_temp
mkdir -p rootfs/var/cache/nginx/fastcgi_temp
mkdir -p rootfs/var/cache/nginx/uwsgi_temp
mkdir -p rootfs/var/cache/nginx/scgi_temp
mkdir -p rootfs/var/log/nginx
mkdir -p rootfs/var/run
mkdir -p rootfs/run

chmod 777 rootfs/run
chmod 777 rootfs/var/run
chmod -R 755 rootfs/var/cache/nginx
chmod -R 755 rootfs/var/log/nginx

echo "Configuring config.json..."

# Use a temporary file for jq edits to avoid truncation
jq '.process.args = ["sleep", "infinity"]' config.json > config.json.tmp && mv config.json.tmp config.json
jq '.process.terminal = false' config.json > config.json.tmp && mv config.json.tmp config.json
jq '.root.readonly = false' config.json > config.json.tmp && mv config.json.tmp config.json

CAPS='["CAP_CHOWN", "CAP_SETGID", "CAP_SETUID", "CAP_NET_BIND_SERVICE"]'
jq ".process.capabilities.bounding += $CAPS" config.json > config.json.tmp && mv config.json.tmp config.json
jq ".process.capabilities.effective += $CAPS" config.json > config.json.tmp && mv config.json.tmp config.json
jq ".process.capabilities.permitted += $CAPS" config.json > config.json.tmp && mv config.json.tmp config.json

echo "Creating the container..."
sudo runc create --bundle $(pwd) ${CONTAINER_ID}

CONTAINER_PID=$(sudo runc state ${CONTAINER_ID} | jq -r '.pid')
echo "Container PID: $CONTAINER_PID"

echo "Setting up networking..."
sudo mkdir -p /run/netns
sudo ln -sfT /proc/${CONTAINER_PID}/ns/net /run/netns/${CONTAINER_ID}

sudo ip link add veth0 type veth peer name ceth0
sudo ip link set ceth0 netns ${CONTAINER_ID}

# Configure container's interface
sudo ip netns exec ${CONTAINER_ID} ip link set ceth0 up
sudo ip netns exec ${CONTAINER_ID} ip addr add 192.168.0.2/24 dev ceth0

# Configure host's interface
sudo ip link set veth0 up
sudo ip addr add 192.168.0.1/24 dev veth0

# Step 6: Start and test the container
echo "Starting the container..."
sudo runc start ${CONTAINER_ID}

# Start nginx inside the container
echo "Starting nginx inside the container..."
sudo runc exec -d ${CONTAINER_ID} nginx -g "daemon off;"

# Give nginx a moment to start
sleep 2

# Test nginx is accessible
echo "Testing nginx connectivity (http://192.168.0.2)..."
curl -s http://192.168.0.2 | head -n 10
