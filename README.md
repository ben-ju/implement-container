sudo apt-get update
sudo apt-get install debootstrap
sudo mkdir /path/to/ubuntu-rootfs
sudo debootstrap focal /path/to/ubuntu-rootfs http://archive.ubuntu.com/ubuntu/
