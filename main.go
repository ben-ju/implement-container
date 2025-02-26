package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

const (
	HOME_DIR = "/home/benju"
	FS_PATH  = HOME_DIR + "/ubuntu-rootfs"
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
	fmt.Printf("Running : %v as %d\n", os.Args[2:], os.Getpid())

	// Créer un répertoire d'info sur l'hôte
	timestamp := time.Now().Unix()
	dirName := fmt.Sprintf("container-info-%d", timestamp)
	hostInfoDir := filepath.Join(HOME_DIR, dirName)
	if err := os.MkdirAll(hostInfoDir, 0777); err != nil {
		fmt.Println("Erreur lors de la création du répertoire hostInfoDir:", err)
		os.Exit(1)
	}
	fmt.Println("Création du répertoire d'info sur l'hôte:", hostInfoDir)

	// Bind-monter hostInfoDir dans FS_PATH/host_info
	bindTarget := filepath.Join(FS_PATH, "host_info")
	if err := os.MkdirAll(bindTarget, 0777); err != nil {
		fmt.Println("Erreur lors de la création du point de montage dans FS_PATH:", err)
		os.Exit(1)
	}
	// On monte le dossier de l'hôte sur le point de montage dans FS_PATH
	if err := syscall.Mount(hostInfoDir, bindTarget, "", syscall.MS_BIND, ""); err != nil {
		fmt.Println("Erreur lors du bind mount:", err)
		os.Exit(1)
	}
	fmt.Println("Bind mount réalisé: ", hostInfoDir, "->", bindTarget)

	// Préparer l'environnement pour transmettre le chemin bindé à l'enfant
	os.Setenv("HOST_INFO_DIR", "/host_info") // Le chemin relatif après chroot

	// Construire la commande pour lancer le processus enfant via systemd-run (optionnel)
	commandList := append([]string{"child"}, os.Args[2:]...)
	cmd := exec.Command("/proc/self/exe", commandList...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
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

	// Définir le hostname dans le conteneur
	if err := syscall.Sethostname([]byte("container")); err != nil {
		fmt.Println("Error setting hostname:", err)
		os.Exit(1)
	}

	// Chroot dans FS_PATH
	if err := syscall.Chroot(FS_PATH); err != nil {
		fmt.Println("Error changing root filesystem:", err)
		os.Exit(1)
	}

	// Changer le répertoire de travail dans le nouveau root
	if err := syscall.Chdir("/"); err != nil {
		fmt.Println("Error changing directory:", err)
		os.Exit(1)
	}

	mountFilesystems()

	// Ici, on utilise la variable d'environnement pour obtenir le point de montage
	hostInfo := os.Getenv("HOST_INFO_DIR")
	if hostInfo == "" {
		fmt.Println("HOST_INFO_DIR non défini")
		os.Exit(1)
	}
	fmt.Println("Utilisation de HOST_INFO_DIR =", hostInfo)

	cgroup(hostInfo)

	// Exécuter la commande passée en argument
	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("Error running child command:", err)
		os.Exit(1)
	}
	unmountFilesystems()
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

func cgroup(hostInfo string) {
	// Ici, on crée un sous-dossier dans le point de montage bindé sur l'hôte.
	// Par exemple, on crée "container" dans le dossier bindé.
	infoDir := filepath.Join(hostInfo, "container")
	if err := os.Mkdir(infoDir, 0775); err != nil && !os.IsExist(err) {
		fmt.Println("Erreur lors de la création du répertoire container dans HOST_INFO_DIR :", err)
		os.Exit(1)
	}

	// On écrit quelques fichiers pour simuler l'information cgroup
	if err := os.WriteFile(filepath.Join(infoDir, "memory.max"), []byte("20"), 0700); err != nil {
		fmt.Println("Error setting memory.max:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(infoDir, "notify_on_release"), []byte("1"), 0700); err != nil {
		fmt.Println("Error setting notify_on_release:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(infoDir, "cgroup.procs"), []byte(strconv.Itoa(os.Getpid())), 0700); err != nil {
		fmt.Println("Error adding process to cgroup:", err)
		os.Exit(1)
	}
	fmt.Println("Répertoire cgroup créé et fichiers écrits dans", infoDir)
}
