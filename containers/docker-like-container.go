//go:build linux

// This is a Go program that demonstrates the fundamental syscalls and kernel features that power containers.
// It's stripped of all abstractions to show you exactly what's happening at the system level.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// This function runs in the PARENT namespace
func run() {
	// os.Args[2:] contains the command to run inside the container (e.g., "/bin/bash")
	// os.Getpid() returns the process ID as seen from the HOST namespace
	//
	// In the parent, this will be something like PID 12345
	// In the child (with CLONE_NEWPID), this will be PID 1
	fmt.Printf("Running %v as PID %d\n", os.Args[2:], os.Getpid())

	// Create the command that will run in new namespaces
	//
	// `/proc/self/exe`: Special symlink that points to the currently running executable which allows the program to re-execute itself
	// `/proc/self/`: is a special directory in Linux that always points to the current process
	// `exe`: is a symlink to the actual executable binary
	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)

	// Redirect stdin, stdout, and stderr to the parent's standard streams. This what makes the container interactive
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// flags to create new namespaces
	// These flags are passed to the Linux clone() syscall. Each flag creates a NEW namespace for the child process
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Creates a new UTS namespace to isolate the hostname and domain name.
		// (UTS = Unix Timesharing System)
		Cloneflags: syscall.CLONE_NEWUTS |
			// Creates a new PID namespace. The child process becomes PID 1 in its own namespace while parent can still see child's real PID.
			syscall.CLONE_NEWPID |
			// Creates a new namespace. Child has its own mount table, isolated from parent(host).
			syscall.CLONE_NEWNS |
			// Creates a new network namespace. The child process has its own network stack. (You have to use veth to connect to the parent's network)
			syscall.CLONE_NEWNET |
			// Creates a new IPC namespace(Inter-Process Communication) objects. The child process has its own IPC objects, isolated from parent(host).
			syscall.CLONE_NEWIPC,
		// Unshareflags: applied AFTER the process is created but BEFORE exec.
		// `CLONE_NEWNS`: ensures mount changes don't propagate to the parent(host).
		Unshareflags: syscall.CLONE_NEWNS,
	}

	if err := cmd.Run(); err != nil {
		panic(err)
	}
}

func child() {
	fmt.Printf("Running %v as PID %d\n", os.Args[2:], os.Getpid())

	// Setup cgroup for memory limit
	cgroups()

	// Change hostname (proving UTS namespace isolation)
	if err := syscall.Sethostname([]byte("container")); err != nil {
		panic(err)
	}

	// Change root filesystem (pivot_root would be more correct)
	if err := syscall.Chroot("/rootfs"); err != nil {
		panic(err)
	}
	if err := os.Chdir("/"); err != nil {
		panic(err)
	}

	// Mount proc filesystem
	if err := syscall.Mount("proc", "proc", "proc", 0, ""); err != nil {
		panic(err)
	}

	// Execute the actual command
	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		panic(err)
	}

	// Cleanup
	if err := syscall.Unmount("proc", 0); err != nil {
		panic(err)
	}
}

func cgroups() {
	// Try cgroups v2 first (unified hierarchy), then fall back to v1
	cgroupV2Path := "/sys/fs/cgroup/mycontainer"
	cgroupV1Path := "/sys/fs/cgroup/memory/mycontainer"

	// Check if cgroups v2 is available
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
		// cgroups v2
		os.Mkdir(cgroupV2Path, 0755)

		// Limit memory to 100MB (cgroups v2 uses memory.max)
		if err := os.WriteFile(cgroupV2Path+"/memory.max", []byte("100000000"), 0700); err != nil {
			fmt.Printf("Warning: could not set memory limit: %v\n", err)
		}

		// Add current process to cgroup
		if err := os.WriteFile(cgroupV2Path+"/cgroup.procs", []byte(fmt.Sprintf("%d", os.Getpid())), 0700); err != nil {
			fmt.Printf("Warning: could not add process to cgroup: %v\n", err)
		}
	} else {
		// cgroups v1
		os.Mkdir(cgroupV1Path, 0755)

		// Limit memory to 100MB (cgroups v1 uses memory.limit_in_bytes)
		if err := os.WriteFile(cgroupV1Path+"/memory.limit_in_bytes", []byte("100000000"), 0700); err != nil {
			fmt.Printf("Warning: could not set memory limit: %v\n", err)
		}

		// Add current process to cgroup
		if err := os.WriteFile(cgroupV1Path+"/cgroup.procs", []byte(fmt.Sprintf("%d", os.Getpid())), 0700); err != nil {
			fmt.Printf("Warning: could not add process to cgroup: %v\n", err)
		}
	}
}

// Main function - this runs in the parent namespace
func main() {
	switch os.Args[1] {
	case "run":
		run() // Initial invocation by the user (parent process)
	case "child":
		child() //Re-execution of itself in new namespaces (child process)
	default:
		panic("bad command")
	}
}
