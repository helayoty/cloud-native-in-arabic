# DEEP DIVE: LINE-BY-LINE CODE EXPLANATION
## Building a Container Runtime from Scratch in Go

This document provides an exhaustive explanation of every line in the minimal container runtime implementation.

---

## OVERVIEW

This is a ~100-line Go program that demonstrates the fundamental syscalls and kernel features that power containers. It's stripped of all abstractions to show you exactly what's happening at the system level.

**What this demonstrates:**
- Namespace creation (process isolation)
- Cgroup setup (resource limits)
- Filesystem manipulation (chroot)
- Process execution in isolated environments

---

## FULL CODE WITH LINE NUMBERS

```go
 1  package main
 2  
 3  import (
 4      "fmt"
 5      "os"
 6      "os/exec"
 7      "syscall"
 8  )
 9  
10  // Main function - this runs in the parent namespace
11  func main() {
12      switch os.Args[1] {
13      case "run":
14          run()
15      case "child":
16          child()
17      default:
18          panic("bad command")
19      }
20  }
21  
22  func run() {
23      fmt.Printf("Running %v as PID %d\n", os.Args[2:], os.Getpid())
24      
25      // Create the command that will run in new namespaces
26      cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
27      cmd.Stdin = os.Stdin
28      cmd.Stdout = os.Stdout
29      cmd.Stderr = os.Stderr
30      
31      // CRITICAL: These flags create new namespaces
32      cmd.SysProcAttr = &syscall.SysProcAttr{
33          Cloneflags: syscall.CLONE_NEWUTS |   // Hostname
34                     syscall.CLONE_NEWPID |    // Process IDs
35                     syscall.CLONE_NEWNS |     // Mount points
36                     syscall.CLONE_NEWNET |    // Network
37                     syscall.CLONE_NEWIPC,     // IPC
38          Unshareflags: syscall.CLONE_NEWNS,
39      }
40      
41      must(cmd.Run())
42  }
43  
44  func child() {
45      fmt.Printf("Running %v as PID %d\n", os.Args[2:], os.Getpid())
46      
47      // Setup cgroup for memory limit (simplified)
48      cgroups()
49      
50      // Change hostname (proving UTS namespace isolation)
51      must(syscall.Sethostname([]byte("container")))
52      
53      // Change root filesystem (pivot_root would be more correct)
54      must(syscall.Chroot("/path/to/rootfs"))
55      must(os.Chdir("/"))
56      
57      // Mount proc filesystem
58      must(syscall.Mount("proc", "proc", "proc", 0, ""))
59      
60      // Execute the actual command
61      cmd := exec.Command(os.Args[2], os.Args[3:]...)
62      cmd.Stdin = os.Stdin
63      cmd.Stdout = os.Stdout
64      cmd.Stderr = os.Stderr
65      
66      must(cmd.Run())
67      
68      // Cleanup
69      must(syscall.Unmount("proc", 0))
70  }
71  
72  func cgroups() {
73      cgroupPath := "/sys/fs/cgroup/memory/mycontainer"
74      os.Mkdir(cgroupPath, 0755)
75      
76      // Limit memory to 100MB
77      must(os.WriteFile(cgroupPath+"/memory.limit_in_bytes", []byte("100000000"), 0700))
78      
79      // Add current process to cgroup
80      must(os.WriteFile(cgroupPath+"/cgroup.procs", []byte(fmt.Sprintf("%d", os.Getpid())), 0700))
81  }
82  
83  func must(err error) {
84      if err != nil {
85          panic(err)
86      }
87  }
```

---

## LINE-BY-LINE EXPLANATION

### Lines 1-8: Package and Imports

```go
package main
```
**Declares this as an executable program (main package).**
- In Go, `package main` indicates this is an entry point for an executable
- The compiler will look for a `main()` function to start execution

```go
import (
    "fmt"
    "os"
    "os/exec"
    "syscall"
)
```
**Import required standard library packages:**

- **`fmt`**: Formatted I/O (printing to console)
- **`os`**: Operating system functions (file operations, process info, environment)
- **`os/exec`**: Execute external commands and manage processes
- **`syscall`**: Direct access to low-level operating system primitives
  - This is the KEY package - it provides raw access to Linux kernel features
  - Contains constants and functions for namespaces, chroot, mount, etc.

---

### Lines 10-20: Main Entry Point

```go
func main() {
```
**Program entry point - executed when binary runs.**
- Every executable Go program must have exactly one `main()` function in package `main`

```go
    switch os.Args[1] {
```
**Command-line argument parsing.**
- `os.Args` is a slice containing all command-line arguments
- `os.Args[0]` is the program name itself (e.g., `./container`)
- `os.Args[1]` is the first argument after the program name
- This creates a simple command router: `./container run /bin/bash` or `./container child /bin/bash`

**Why two modes?**
- `run`: Initial invocation by the user (parent process)
- `child`: Re-execution of itself in new namespaces (child process)
- This pattern allows the program to create namespaces by exec'ing itself

```go
    case "run":
        run()
    case "child":
        child()
    default:
        panic("bad command")
    }
```
**Route execution based on the command:**
- If first arg is "run", call `run()` function
- If first arg is "child", call `child()` function  
- Otherwise, crash with error message

---

### Lines 22-42: The `run()` Function (Parent Process)

This function runs in the PARENT namespace (the normal system namespace where you started the program).

```go
func run() {
    fmt.Printf("Running %v as PID %d\n", os.Args[2:], os.Getpid())
```
**Print what we're about to run and our current PID.**
- `os.Args[2:]` slices from index 2 to end (the actual command to run in container)
  - Example: if you ran `./container run /bin/bash`, `os.Args[2:]` = `["/bin/bash"]`
- `os.Getpid()` returns the process ID in the current PID namespace
  - In the parent, this will be something like PID 12345
  - In the child (with CLONE_NEWPID), this will be PID 1

```go
    // Create the command that will run in new namespaces
    cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
```
**Create a command to execute ourselves with "child" argument.**

Let me break down this complex line:

- **`/proc/self/exe`**: Special symlink that points to the currently running executable
  - This allows the program to re-execute itself
  - `/proc/self/` is a special directory in Linux that always points to the current process
  - `exe` is a symlink to the actual executable binary
  
- **`append([]string{"child"}, os.Args[2:]...)`**: Build the argument list
  - Start with `[]string{"child"}` - this will be os.Args[1] in the child
  - Append `os.Args[2:]...` - the user's command (e.g., `/bin/bash`)
  - The `...` operator unpacks the slice
  - Result: `["child", "/bin/bash"]` if original was `["./container", "run", "/bin/bash"]`

**Example transformation:**
```
Original:  ./container run /bin/bash
Parent:    os.Args = ["./container", "run", "/bin/bash"]
Child:     os.Args = ["/proc/self/exe", "child", "/bin/bash"]
           Which becomes: ["./container", "child", "/bin/bash"] when executed
```

```go
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
```
**Connect parent's standard streams to child process.**
- Without this, the child process would have no way to receive input or display output
- `cmd.Stdin`: Where the child reads input from (keyboard)
- `cmd.Stdout`: Where the child writes normal output to (terminal)
- `cmd.Stderr`: Where the child writes error output to (terminal)

This makes the container interactive - you can type commands and see output.

```go
    // CRITICAL: These flags create new namespaces
    cmd.SysProcAttr = &syscall.SysProcAttr{
```
**Begin configuring syscall attributes for process creation.**
- `SysProcAttr` is a struct that specifies OS-specific process attributes
- The `&` creates a pointer to the struct
- These attributes are passed to the `clone()` syscall under the hood

```go
        Cloneflags: syscall.CLONE_NEWUTS |   // Hostname
                   syscall.CLONE_NEWPID |    // Process IDs
                   syscall.CLONE_NEWNS |     // Mount points
                   syscall.CLONE_NEWNET |    // Network
                   syscall.CLONE_NEWIPC,     // IPC
```
**THE MAGIC: Namespace creation flags.**

These flags are passed to the Linux `clone()` syscall (similar to `fork()` but more powerful).
Each flag creates a NEW namespace for the child process:

**`syscall.CLONE_NEWUTS`** (UTS = Unix Timesharing System):
- Creates a new UTS namespace
- Isolates hostname and domain name
- Child can change hostname without affecting parent
- Demo: We'll set hostname to "container" - it won't affect your host system

**`syscall.CLONE_NEWPID`** (Process ID):
- Creates a new PID namespace
- Child process becomes PID 1 in its own namespace
- Child can only see processes in its own namespace
- Parent can still see child's "real" PID
- This is why `ps aux` in a container only shows container processes

**`syscall.CLONE_NEWNS`** (Mount):
- Creates a new mount namespace
- Child has its own mount table
- Mounting/unmounting in child doesn't affect parent
- Allows us to safely pivot the root filesystem

**`syscall.CLONE_NEWNET`** (Network):
- Creates a new network namespace
- Child starts with NO network interfaces (not even loopback)
- Completely isolated network stack
- In production, you'd use veth pairs to connect to host network

**`syscall.CLONE_NEWIPC`** (Inter-Process Communication):
- Creates a new IPC namespace
- Isolates System V IPC objects (message queues, semaphores, shared memory)
- Prevents cross-namespace IPC

**Missing namespaces (not used here):**
- `CLONE_NEWUSER`: User namespace (UID/GID isolation)
- `CLONE_NEWCGROUP`: Cgroup namespace (cgroup visibility isolation)

**The `|` operator:**
- Bitwise OR - combines multiple flags into one bitmask
- The kernel checks which bits are set to determine which namespaces to create

```go
        Unshareflags: syscall.CLONE_NEWNS,
    }
```
**Additional unshare operation for mount namespace.**
- `Unshareflags` are applied AFTER the process is created but BEFORE exec
- `CLONE_NEWNS` here ensures mount changes don't propagate
- This is needed because mount namespaces have special sharing semantics
- Without this, mounts might still propagate due to mount point propagation types

```go
    must(cmd.Run())
}
```
**Execute the child process and wait for it to complete.**
- `cmd.Run()` starts the process and blocks until it exits
- It combines `cmd.Start()` (create process) and `cmd.Wait()` (wait for exit)
- `must()` is our error-checking helper (panics if error)

At this point, the kernel:
1. Creates new namespaces per the Cloneflags
2. Calls `clone()` syscall to create child process
3. Child process starts running in new namespaces
4. Child executes `/proc/self/exe child /bin/bash`
5. Child's `main()` function runs and calls `child()`

---

### Lines 44-70: The `child()` Function (Container Process)

This function runs INSIDE the new namespaces. It's isolated from the parent system.

```go
func child() {
    fmt.Printf("Running %v as PID %d\n", os.Args[2:], os.Getpid())
```
**Print our command and PID - should show PID 1 due to CLONE_NEWPID.**

CRITICAL OBSERVATION:
- In the parent, `os.Getpid()` returned something like 12345
- In the child, `os.Getpid()` returns 1 (or 2 in some cases)
- This proves we're in a new PID namespace
- The child is the init process of its namespace

```go
    // Setup cgroup for memory limit (simplified)
    cgroups()
```
**Call the cgroups setup function (explained below).**
- This limits the memory available to our container
- Must be done before executing the target command

```go
    // Change hostname (proving UTS namespace isolation)
    must(syscall.Sethostname([]byte("container")))
```
**Change the hostname to "container".**

**`syscall.Sethostname([]byte("container"))`:**
- Direct syscall to Linux kernel `sethostname()`
- Requires a byte slice ([]byte) as input
- Changes the hostname for the current UTS namespace
- In parent namespace, hostname remains unchanged (isolation proof!)

You can verify: run `hostname` inside and outside the container - they'll be different.

```go
    // Change root filesystem (pivot_root would be more correct)
    must(syscall.Chroot("/path/to/rootfs"))
```
**Change root directory (chroot jail).**

**What is chroot?**
- `chroot` = "change root"
- System call that changes the root directory for the current process and its children
- Everything the process sees starts from the new root
- `/path/to/rootfs` should contain a complete filesystem (bin, lib, etc.)

**How it works:**
- Before chroot: `/bin/bash` points to host's `/bin/bash`
- After chroot to `/path/to/rootfs`: `/bin/bash` points to `/path/to/rootfs/bin/bash`
- Process is "jailed" - it cannot access files outside the new root

**Real-world setup:**
```bash
# Create a minimal rootfs
mkdir -p /tmp/rootfs/{bin,lib,lib64,proc}

# Copy bash and its dependencies
cp /bin/bash /tmp/rootfs/bin/
cp /lib/x86_64-linux-gnu/libtinfo.so.* /tmp/rootfs/lib/
cp /lib/x86_64-linux-gnu/libdl.so.* /tmp/rootfs/lib/
cp /lib/x86_64-linux-gnu/libc.so.* /tmp/rootfs/lib/
cp /lib64/ld-linux-x86-64.so.* /tmp/rootfs/lib64/

# Use this as your chroot path
must(syscall.Chroot("/tmp/rootfs"))
```

**Why not pivot_root?**
- `pivot_root` is more correct for containers (moves the root, doesn't just change view)
- `chroot` is simpler for demonstration but has security limitations
- `chroot` can be escaped by root processes with right capabilities
- `pivot_root` combined with unmounting the old root is more secure

```go
    must(os.Chdir("/"))
```
**Change current directory to the new root.**

After `chroot`, our working directory is still outside the chroot jail (weird quirk).
We must explicitly cd to "/" to be fully inside the jail.

**Why this matters:**
- Without this, file operations might still reference the old filesystem
- The process could potentially escape the chroot by following .. paths

```go
    // Mount proc filesystem
    must(syscall.Mount("proc", "proc", "proc", 0, ""))
```
**Mount the proc filesystem.**

**Understanding this mount call:**

```go
syscall.Mount(source, target, fstype, flags, data)
```

- **source** ("proc"): What to mount (special keyword for procfs)
- **target** ("proc"): Where to mount it (relative path, so /proc after chroot)
- **fstype** ("proc"): Filesystem type (procfs is a virtual filesystem)
- **flags** (0): No special mount flags
- **data** (""): No additional mount options

**What is /proc?**
- Virtual filesystem provided by the kernel
- Exposes process and system information as files
- Examples: `/proc/cpuinfo`, `/proc/meminfo`, `/proc/[pid]/`
- Many tools (ps, top, htop) read from /proc

**Why mount it in container?**
- Without /proc, tools like `ps` won't work
- Each mount namespace needs its own proc mount
- The proc we mount here only shows processes in our PID namespace

**Verify isolation:**
```bash
# Outside container
ps aux  # Shows ALL system processes

# Inside container  
ps aux  # Shows only container processes (because /proc is isolated)
```

```go
    // Execute the actual command
    cmd := exec.Command(os.Args[2], os.Args[3:]...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
```
**Prepare to execute the target command (e.g., /bin/bash).**

- `os.Args[2]` is the command (e.g., "/bin/bash")
- `os.Args[3:]` are additional arguments (if any)
- Wire up standard streams for interactivity

Example:
```
Original command: ./container run /bin/bash -l
Child receives: ["child", "/bin/bash", "-l"]
This creates: exec.Command("/bin/bash", "-l")
```

```go
    must(cmd.Run())
```
**Execute the target command and wait for it to exit.**

This is where your actual application runs - inside all the isolation we've set up:
- New PID namespace (isolated process tree)
- New UTS namespace (custom hostname)
- New mount namespace (private mounts)
- New network namespace (no network)
- New IPC namespace (isolated IPC)
- Chrooted filesystem
- Cgroup resource limits

When this command exits, the container terminates.

```go
    // Cleanup
    must(syscall.Unmount("proc", 0))
}
```
**Unmount /proc before exiting.**

- Good practice to clean up mounts
- `0` means no special unmount flags (could use `MNT_DETACH` for lazy unmount)
- Since we're in our own mount namespace, this doesn't affect the parent

---

### Lines 72-81: The `cgroups()` Function (Resource Limits)

```go
func cgroups() {
    cgroupPath := "/sys/fs/cgroup/memory/mycontainer"
```
**Define the cgroup path for our container.**

**Understanding cgroup filesystem:**
- `/sys/fs/cgroup` is a special filesystem (cgroup filesystem type)
- Each cgroup subsystem has its own directory (memory, cpu, cpuset, etc.)
- Creating directories creates cgroups
- Writing to files in those directories configures limits

**Directory structure:**
```
/sys/fs/cgroup/
├── memory/           # Memory controller
│   ├── mycontainer/  # Our cgroup (we create this)
│   │   ├── memory.limit_in_bytes  # Memory limit
│   │   ├── cgroup.procs           # PIDs in this cgroup
│   │   └── ...
├── cpu/              # CPU controller
├── cpuset/           # CPU/memory node assignment
└── ...
```

```go
    os.Mkdir(cgroupPath, 0755)
```
**Create the cgroup directory.**

- Creates `/sys/fs/cgroup/memory/mycontainer`
- The kernel automatically creates the control files inside this directory
- `0755` = rwxr-xr-x permissions

```go
    // Limit memory to 100MB
    must(os.WriteFile(cgroupPath+"/memory.limit_in_bytes", []byte("100000000"), 0700))
```
**Set the memory limit to 100MB.**

**Breaking down this cgroup configuration:**

- **`cgroupPath+"/memory.limit_in_bytes"`**: Full path to the limit file
  - This is a special kernel interface file
  - Writing to it configures the memory controller
  
- **`[]byte("100000000")`**: 100,000,000 bytes = ~95 MB
  - Must be written as bytes (ASCII string representation of number)
  - Kernel parses this and enforces the limit
  
- **What happens when limit is exceeded:**
  - Kernel triggers OOM (Out of Memory) killer
  - OOM killer selects and terminates processes in the cgroup
  - Container crashes if it tries to use more than 100MB

**Real-world cgroup configuration:**
```bash
# These are the files you can configure in cgroups v1:

# Memory limits
echo 100000000 > memory.limit_in_bytes           # Hard limit
echo 80000000 > memory.soft_limit_in_bytes       # Soft limit (reclaim under pressure)

# CPU limits  
echo 50000 > cpu.cfs_quota_us    # 50ms per period
echo 100000 > cpu.cfs_period_us  # 100ms period (50% of one CPU)

# Block I/O limits
echo "8:0 1048576" > blkio.throttle.read_bps_device  # 1MB/s read limit
```

```go
    // Add current process to cgroup
    must(os.WriteFile(cgroupPath+"/cgroup.procs", []byte(fmt.Sprintf("%d", os.Getpid())), 0700))
}
```
**Add the current process (and all children) to the cgroup.**

**How process assignment works:**

- **`cgroup.procs`**: Special file that lists PIDs in the cgroup
- **Writing a PID**: Moves that process (and all its threads) into the cgroup
- **`os.Getpid()`**: Gets our current process ID
- **`fmt.Sprintf("%d", ...)`**: Converts PID to string (kernel expects ASCII)

**What happens after this:**
- Our process is now under cgroup control
- All children we spawn inherit cgroup membership
- Memory usage is tracked and limited by the kernel
- Any memory allocation beyond 100MB will fail or trigger OOM

**Key point:** The limit applies to the entire process tree (parent + all children).

---

### Lines 83-87: Error Handling Helper

```go
func must(err error) {
    if err != nil {
        panic(err)
    }
}
```
**Simple error checking - panic if there's an error.**

- **`error` type**: Go's built-in error interface
- **`nil`**: No error occurred
- **`panic(err)`**: Crash the program with the error message

This is simplified for demonstration. Production code would:
- Log errors properly
- Clean up resources (unmount, remove cgroups)
- Return errors instead of panicking
- Provide meaningful error messages

---

## SYSCALL DEEP DIVE

### clone() Syscall (via cmd.Run() with SysProcAttr)

When you call `cmd.Run()` with namespace flags, Go uses the `clone()` syscall:

```c
// Actual Linux syscall signature
long clone(unsigned long flags, void *child_stack,
           void *ptid, void *ctid, struct pt_regs *regs);
```

**What clone does:**
1. Creates a new process (like fork)
2. Selectively shares or isolates resources based on flags
3. Returns twice - once in parent (child PID), once in child (0)

**Namespace flags we use:**
```c
CLONE_NEWUTS    0x04000000  // New UTS namespace
CLONE_NEWPID    0x20000000  // New PID namespace  
CLONE_NEWNS     0x00020000  // New mount namespace
CLONE_NEWNET    0x40000000  // New network namespace
CLONE_NEWIPC    0x08000000  // New IPC namespace
```

These are bitwise OR'd together into a single flags value.

---

### Kernel's Namespace Creation Process

When clone() is called with CLONE_NEW* flags:

1. **Kernel validates**:
   - Does process have CAP_SYS_ADMIN capability?
   - Are namespace features enabled in kernel config?

2. **Kernel creates new namespace structures**:
   ```c
   struct uts_namespace *uts_ns = create_uts_ns();
   struct pid_namespace *pid_ns = create_pid_namespace();
   struct mnt_namespace *mnt_ns = create_mnt_namespace();
   struct net *net_ns = create_net_namespace();
   struct ipc_namespace *ipc_ns = create_ipc_namespace();
   ```

3. **Kernel associates process with new namespaces**:
   ```c
   task->nsproxy->uts_ns = uts_ns;
   task->nsproxy->pid_ns_for_children = pid_ns;
   task->nsproxy->mnt_ns = mnt_ns;
   task->nsproxy->net_ns = net_ns;
   task->nsproxy->ipc_ns = ipc_ns;
   ```

4. **Child process starts**:
   - Has pointer to its namespace structures
   - Syscalls check task->nsproxy->*_ns to determine visibility

---

### chroot() Syscall

```c
int chroot(const char *path);
```

**Kernel operation:**
1. Validates path exists and is directory
2. Changes current task's `fs->root` to new dentry
3. All path resolution starts from new root

**Security note:**
- chroot is NOT sufficient security
- Processes with CAP_SYS_CHROOT can chroot again and escape
- Must combine with other isolation (namespaces, capabilities, seccomp)

**Escape example (simplified):**
```c
mkdir("breakout", 0755);
chroot("breakout");  // Chroot to empty dir
// Now .. still points outside, can navigate up
```

This is why pivot_root is preferred - it doesn't have this issue.

---

### mount() Syscall

```c
int mount(const char *source, const char *target,
          const char *filesystemtype, unsigned long mountflags,
          const void *data);
```

**For procfs:**
```c
mount("proc", "/proc", "proc", 0, NULL);
```

**Kernel operation:**
1. Looks up filesystem type in registered filesystems
2. Calls proc_mount() function from procfs driver
3. Creates new superblock for procfs
4. Allocates inodes for /proc entries
5. Populates with current namespace's processes

**Why procfs needs mounting in each namespace:**
- procfs shows PIDs from the current PID namespace
- Each mount has different view based on namespace
- This is how `ps` only sees container processes

---

## ADVANCED TOPICS

### What We Skipped (Production Requirements)

**1. User Namespaces (CLONE_NEWUSER):**
```go
Cloneflags: syscall.CLONE_NEWUSER,
```
- Maps UIDs/GIDs between namespaces
- Allows rootless containers
- UID 0 in container != UID 0 on host

**2. Capabilities:**
```go
cmd.SysProcAttr.AmbientCaps = []uintptr{
    CAP_NET_BIND_SERVICE, // Bind to port < 1024
    // Drop all others
}
```
- Fine-grained privileges instead of root/non-root
- Containers should run with minimal capabilities

**3. Seccomp (Syscall Filtering):**
```go
// Allow only safe syscalls
seccomp.LoadProfile(allowList)
```
- Blocks dangerous syscalls (reboot, load kernel modules, etc.)
- Essential security layer

**4. pivot_root instead of chroot:**
```go
syscall.PivotRoot(newroot, putold)
syscall.Unmount(putold, syscall.MNT_DETACH)
```
- More secure than chroot
- Completely replaces root, no escape path

**5. Network Configuration:**
```go
// Create veth pair
link, _ := netlink.LinkAdd(&netlink.Veth{
    LinkAttrs: netlink.LinkAttrs{Name: "veth0"},
    PeerName: "veth1",
})

// Move one end to container namespace  
netlink.LinkSetNsPid(link, containerPid)
```

**6. Cgroup v2:**
```go
// Modern unified cgroup hierarchy
cgroupPath := "/sys/fs/cgroup/mycontainer"
os.WriteFile(cgroupPath+"/cgroup.controllers", []byte("+memory +cpu"), 0644)
os.WriteFile(cgroupPath+"/memory.max", []byte("100M"), 0644)
os.WriteFile(cgroupPath+"/cpu.max", []byte("50000 100000"), 0644)
```

**7. Image Layers (Union Filesystem):**
```go
// Mount OverlayFS
syscall.Mount("overlay", "/merged", "overlay", 0,
    "lowerdir=/ro/layer1:/ro/layer2,upperdir=/rw/upper,workdir=/rw/work")
```

---

## TESTING THE CODE

### Build and Run

```bash
# Build the container runtime
go build -o container main.go

# Create a minimal rootfs (Alpine-based)
mkdir -p /tmp/alpine-rootfs
cd /tmp/alpine-rootfs
wget https://dl-cdn.alpinelinux.org/alpine/v3.19/releases/x86_64/alpine-minirootfs-3.19.0-x86_64.tar.gz
tar -xzf alpine-minirootfs-3.19.0-x86_64.tar.gz
rm alpine-minirootfs-3.19.0-x86_64.tar.gz

# Update the code to use this rootfs
# Change line 54 to: must(syscall.Chroot("/tmp/alpine-rootfs"))

# Run as root (required for namespace operations)
sudo ./container run /bin/sh
```

### Verify Isolation

Inside the container:
```bash
# Check PID (should be 1 or 2)
echo $$

# Check hostname
hostname  # Should show "container"

# Check processes (only container processes visible)
ps aux

# Check memory limit is enforced
cat /proc/self/cgroup  # Shows you're in mycontainer cgroup
cat /sys/fs/cgroup/memory/mycontainer/memory.limit_in_bytes

# Try to use > 100MB memory
# (Use a memory-eating program or stress test)
```

Outside the container (different terminal):
```bash
# See the container process
ps aux | grep container

# Check actual cgroup membership
cat /proc/$(pidof container)/cgroup

# Monitor cgroup memory usage
watch cat /sys/fs/cgroup/memory/mycontainer/memory.usage_in_bytes
```

---

## COMPARISON TO DOCKER

### What Docker Adds

Our minimal runtime is ~100 lines. Docker (actually runc) adds:

1. **Image Management**:
   - Download and cache images
   - Layer storage (OverlayFS)
   - Image building

2. **Network Configuration**:
   - veth pairs
   - Bridge networks  
   - Port forwarding
   - DNS

3. **Storage**:
   - Volume management
   - Bind mounts
   - tmpfs

4. **Security**:
   - AppArmor/SELinux profiles
   - Seccomp filters
   - Capability dropping
   - User namespace mapping

5. **Monitoring**:
   - Log collection
   - Stats/metrics
   - Health checks

6. **Orchestration**:
   - Lifecycle management
   - Restart policies
   - Inter-container networking

