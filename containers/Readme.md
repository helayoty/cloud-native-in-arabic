
# Create Container in Linux VM and Docker Networking Demos

## Option 1: Access Docker Desktop VM

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

### Using Multipass

#### Step 1: Install multipass

```bash
# Install Multipass
brew install multipass

# Create Ubuntu VM
multipass launch --name docker-vm --cpus 2 --memory 2G --disk 10G

# Shell into VM
multipass shell docker-vm
```

#### Step 2: Install docker inside VM

```bash
sudo apt update
sudo apt install -y docker.io docker-compose jq bridge-utils

# Add user to docker group
sudo usermod -aG docker $USER
newgrp docker
```

#### (Optional) Step 3: Copy files to VM

```bash
multipass transfer <FILE_NAME_HERE> docker-vm:

chmod +x <FILE_NAME_HERE>
```

---

## Running the Container Demo

This demo shows how Linux namespaces and cgroups work - the building blocks of containers.

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
