
# Create Container in Linux VM and Docker Networking Demos

## Option 1: Run Linux image via Docker

This setup will show you the basic container setup. However, it won't let you experinse the networking and other advanced setup.

### Step 1: Verify Docker Desktop is Running

```bash
# Should show running containers or empty list (not an error)
docker ps
```

### Access the VM

```bash
# Direct access to Docker VM
docker run -it --rm --privileged --net=host --pid=host alpine sh
```

Where:
* --privileged: Grants access to host resources
* --net=host: Shares network namespace with Docker VM
* --pid=host: Shares PID namespace with Docker VM


### Step 3: Install Required Tools

```bash
# Update package index
apk update

# Install networking tools
apk add --no-cache iproute2 iptables bridge-utils tcpdump bash curl wget jq
```

Where:
* iproute2: Provides `ip` for managing network interfaces, routes, and veth pairs
* iptables: Firewall and NAT rule management (inspect Docker's port forwarding)
* bridge-utils: Provides `brctl` for inspecting docker0 bridge connections
* tcpdump: Capture and analyze network traffic between containers
* bash: Shell with better scripting support than sh
* curl: Make HTTP requests to test container connectivity

* (optional) wget: Download files and test network access
* (optional)jq: Parse JSON output from Docker commands

### Step 4: Check everything is working

```bash
ip link show docker0
brctl show docker0
iptables -t nat -L DOCKER
iptables -t nat -L POSTROUTING
iptables -L FORWARD
tcpdump -i docker0
tcpdump -i veth123abc
bridge fdb show
nsenter -t 1 -n ip link show
```

Where:
* `ip link show docker0`: Display the docker0 bridge interface and its state
* `brctl show docker0`: Show which veth interfaces are connected to the bridge
* `iptables -t nat -L DOCKER`: List port publishing rules for container port mappings
* `iptables -t nat -L POSTROUTING`: List MASQUERADE rules for outbound container traffic
* `iptables -L FORWARD`: List forwarding rules between containers and external networks
* `tcpdump -i docker0`: Capture packets on the docker0 bridge
* `tcpdump -i veth123abc`: Capture packets on a specific veth interface
* `bridge fdb show`: Display MAC address forwarding database entries
* `nsenter -t 1 -n ip link show`: Enter PID 1's network namespace and list interfaces

---

## Option 2: Use a Linux VM (For Full Control)

Run a complete Linux VM on macOS for the most authentic experience.

### Using Lima

#### Step 1: Install lima

```bash
# Install Multipass
brew install lima

limactl shell default
```

#### Step 2: Install docker inside VM

```bash
sudo apt update
sudo apt install -y runc docker.io docker-compose docker-cli jq bridge-utils net-tools

# Add user to docker group
sudo usermod -aG docker $USER
newgrp docker
```

#### Setup the Container

```bash
export CONTAINER_ID=container-1

mkdir -p ~/${CONTAINER_ID}
cd ~/${CONTAINER_ID}

# Generate OCI spec
runc spec

# Verify config created
file config.json

# Create rootfs directory
mkdir rootfs

# Pull and export nginx image
export IMAGE=ghcr.io/iximiuz/labs/nginx:alpine
docker pull $IMAGE
docker export $(docker create $IMAGE) | tar -C rootfs -xf -

# Verify rootfs extraction
ls rootfs/

# Fix nginx directory

# Create required directories and set permissions
mkdir -p rootfs/var/cache/nginx/client_temp
mkdir -p rootfs/var/cache/nginx/proxy_temp
mkdir -p rootfs/var/cache/nginx/fastcgi_temp
mkdir -p rootfs/var/cache/nginx/uwsgi_temp
mkdir -p rootfs/var/cache/nginx/scgi_temp
mkdir -p rootfs/var/log/nginx
mkdir -p rootfs/var/run
mkdir -p rootfs/run

# Fix permissions for nginx
chmod 777 rootfs/run
chmod 777 rootfs/var/run
chmod -R 755 rootfs/var/cache/nginx
chmod -R 755 rootfs/var/log/nginx

# ============================================
# CONFIGURE CONTAINER
# ============================================

# Use sleep as init process (nginx will be started via exec)
echo $(jq '.process.args = ["sleep", "infinity"]' config.json) > config.json

# Disable terminal
echo $(jq '.process.terminal = false' config.json) > config.json

# Make rootfs writable
echo $(jq '.root.readonly = false' config.json) > config.json

# Add capabilities
CAPS='["CAP_CHOWN", "CAP_SETGID", "CAP_SETUID", "CAP_NET_BIND_SERVICE"]'
echo $(jq ".process.capabilities.bounding += $CAPS" config.json) > config.json
echo $(jq ".process.capabilities.effective += $CAPS" config.json) > config.json
echo $(jq ".process.capabilities.permitted += $CAPS" config.json) > config.json
```

### Step 4: Create the container

```bash
sudo runc create --bundle $(pwd) ${CONTAINER_ID}

# Get container PID
CONTAINER_PID=$(sudo runc state ${CONTAINER_ID} | jq -r '.pid')
echo "Container PID: $CONTAINER_PID"
```
### Step 5: Setup Networking

```bash
# Create netns directory and symlink
sudo mkdir -p /run/netns
sudo ln -sT /proc/${CONTAINER_PID}/ns/net /run/netns/${CONTAINER_ID}

# Create veth pair
sudo ip link add veth0 type veth peer name ceth0
sudo ip link set ceth0 netns ${CONTAINER_ID}

# Configure container's interface
sudo ip netns exec ${CONTAINER_ID} ip link set ceth0 up
sudo ip netns exec ${CONTAINER_ID} ip addr add 192.168.0.2/24 dev ceth0

# Configure host's interface
sudo ip link set veth0 up
sudo ip addr add 192.168.0.1/24 dev veth0

# Verify networking is configured
sudo ip netns exec ${CONTAINER_ID} ip addr
```

### Step 6: start and test the container

```bash
# Start container
sudo runc start ${CONTAINER_ID}

# Verify container is running
sudo runc list

# ============================================
# START NGINX
# ============================================

# Check nginx is in the container
sudo runc exec ${CONTAINER_ID} which nginx

# Try starting nginx interactively to see errors
sudo runc exec -it ${CONTAINER_ID} sh

# Inside container:
nginx -g "daemon off;" &
netstat -tlnp
ps aux
exit

# ============================================
# TEST
# ============================================

# Test nginx is accessible (from Lima VM)
curl -s http://192.168.0.2

# Should see nginx welcome page HTML

# Test with verbose output
# curl -v http://192.168.0.2

```

### Step 7: (Optional) Interactive Testing

```bash
# Enter container interactively
sudo runc exec -it ${CONTAINER_ID} sh

ps aux
ip addr
netstat -tlnp

exit
```
### Step 8: Clean Up

```bash
# Kill container
sudo runc kill ${CONTAINER_ID} TERM

# Wait a moment for it to stop
sleep 2

# Delete container
sudo runc delete ${CONTAINER_ID}

# Clean up networking
sudo ip link delete veth0
sudo rm -f /run/netns/${CONTAINER_ID}

# Remove bundle directory
cd ~
rm -rf ${CONTAINER_ID}
```
---

## Running the Container Golang Demo

This demo shows how Linux namespaces and cgroups work and the building blocks of containers.

### Step 1: Cross-compile on macOS

```bash
cd containers
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o container docker-like-container.go
chmod +x container
```

### Step 2: Run in a privileged container

```bash
docker run -it --rm --privileged -v $(pwd)/container:/container alpine sh
```
### Step 3: check the current container

```bash
hostname
ps aux
echo $$
```

### Step 4: Set up a rootfs and run the demo

```bash
mkdir -p /rootfs
wget -qO- https://dl-cdn.alpinelinux.org/alpine/v3.19/releases/x86_64/alpine-minirootfs-3.19.0-x86_64.tar.gz | tar xz -C /rootfs
/container/container run /bin/sh
```

The rootfs (root filesystem) is needed because of this line in the code:
    syscall.Chroot("/rootfs")

What chroot does:
It changes what the process sees as / (root directory). After chroot, your container process can't see or access anything outside /rootfs.

Why you need a complete filesystem:
Without it, after chroot your container would have nothing - no /bin/sh, no commands, no libraries. The Alpine minirootfs (~3MB) provides:
```bash
/rootfs/
├── bin/       ← Basic commands (sh, ls, cat...)
├── lib/       ← Shared libraries
├── etc/       ← Configuration files
├── usr/       ← More binaries
└── ...
```

This is filesystem isolation, one of the key container features:
    
| Without chroot	| With chroot to /rootfs|
|-------------------|-----------------------|
| Container sees host's entire filesystem	| Container only sees Alpine's minimal filesystem  |
| Can access /etc/passwd, /home, etc.	    | Isolated - can't escape /rootfs   |

> Real Docker does the same thing, each container image (alpine, ubuntu, nginx) is essentially a rootfs that gets chroot'd into.

### Step 4: Check if container is correct

```bash
hostname
ps aux
echo $$  #   (shell's PID)
```

You should see:
* Hostname becomes "container" - proving UTS namespace isolation
* PID changes from a high number (parent) to 1 (child) - proving PID namespace isolation

```bash
$ ps aux
   PID   USER     TIME    COMMAND
    1    root      0:00   {container} /proc/self/exe child /bin/sh  # container "init" process
    7    root      0:00   /bin/sh      # /bin/sh (this interactive shell)
    9    root      0:00   ps aux       # this command (ps aux) 
```

You're now inside your own mini-container!
