package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/containerd/cgroups/v2/cgroup1"
	"github.com/containerd/cgroups/v2/cgroup2"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	HOME_DIR       = "/home/benju"
	FS_PATH        = HOME_DIR + "/ubuntu-rootfs"
	CONTAINERS_DIR = "/home/benju/containers"
)

func main() {
	switch os.Args[1] {
	case "run":
		run()
	case "child":
		child()
	default:
		panic("Bad command")
	}
}

func run() {
	if os.Geteuid() != 0 {
		fmt.Println("Ce programme doit être exécuté en tant que root.")
		os.Exit(1)
	}

	fmt.Printf("Running : %v as %d\n", os.Args[2:], os.Getpid())
	commandList := append([]string{"child"}, os.Args[2:]...)

	cmd := exec.Command("/proc/self/exe", commandList...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Configurer les namespaces (UTS, PID, mount)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		Unshareflags: syscall.CLONE_NEWNS,
	}

	if err := cmd.Run(); err != nil {
		fmt.Println("Error running command:", err)
		os.Exit(1)
	}
}

func child() {
	fmt.Printf("Running : %v as %d\n", os.Args[2:], os.Getpid())

	if err := syscall.Sethostname([]byte("container")); err != nil {
		fmt.Println("Error setting hostname:", err)
		os.Exit(1)
	}

	if err := syscall.Chroot(FS_PATH); err != nil {
		fmt.Println("Error changing root filesystem:", err)
		os.Exit(1)
	}

	if err := syscall.Chdir("/"); err != nil {
		fmt.Println("Error changing directory:", err)
		os.Exit(1)
	}

	mountFilesystems()

	defer unmountFilesystems()

	// cgroup := createCgroupV1()
	cgroup := createCgroupV2()
	fmt.Printf("cgroup : %v", cgroup)

	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("Error running child command:", err)
		os.Exit(1)
	}
}

func mountFilesystems() {
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		fmt.Println("Error mounting /proc:", err)
		os.Exit(1)
	}

	if err := syscall.Mount("sys", "/sys", "sysfs", 0, ""); err != nil {
		fmt.Println("Error mounting /sys:", err)
		os.Exit(1)
	}
}

func unmountFilesystems() {
	if err := syscall.Unmount("/proc", 0); err != nil {
		fmt.Println("Error unmounting /proc:", err)
		os.Exit(1)
	}

	if err := syscall.Unmount("/sys", 0); err != nil {
		fmt.Println("Error unmounting /sys:", err)
		os.Exit(1)
	}
}

func createCgroupV2() *cgroup2.Manager {
	memMax := int64(10)
	containerID := fmt.Sprintf("benju-%d-%d", os.Getpid(), time.Now().UnixNano())
	res := cgroup2.Resources{
		Memory: &cgroup2.Memory{
			Max: &memMax,
		},
	}
	m, err := cgroup2.NewManager("/sys/fs/cgroup", "/"+containerID, &res)
	if err != nil {
		panic(err)
	}
	return m
}

func createCgroupV1() *cgroup1.Cgroup {
	shares := uint64(100)
	containerID := fmt.Sprintf("benju-%d-%d", os.Getpid(), time.Now().UnixNano())

	control, err := cgroup1.New(
		cgroup1.Default,
		cgroup1.StaticPath("/"+containerID),
		&specs.LinuxResources{
			CPU: &specs.LinuxCPU{
				Shares: &shares,
			},
		},
	)
	if err != nil {
		panic(err)
	}
	return &control
}
