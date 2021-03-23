# Kraken Vagrant/Virtualbox Example

## Table of Contents

- [Kraken Vagrant/Virtualbox Example](#kraken-vagrantvirtualbox-example)
  - [Table of Contents](#table-of-contents)
  - [Dependencies](#dependencies)
    - [WSL](#wsl)
  - [Instructions](#instructions)
    - [Get Kraken](#get-kraken)
    - [Setting up host-only networking](#setting-up-host-only-networking)
      - [MacOS network creation](#macos-network-creation)
      - [Linux network creation](#linux-network-creation)
      - [WSL network creation](#wsl-network-creation)
  - [Release the Kraken](#release-the-kraken)
  - [How it works](#how-it-works)
  - [Helper scripts](#helper-scripts)
  - [How to use your own fork/branch](#how-to-use-your-own-forkbranch)
  - [Using custom kraken-build args](#using-custom-kraken-build-args)
  - [How to add things to the image](#how-to-add-things-to-the-image)
    - [Adding a kernel module](#adding-a-kernel-module)
  - [_Now: `$ go get kraken`!_](#now--go-get-kraken)

The contents of this directory can be used to generate a VirtualBox virtual Kraken cluster using Vagrant and Ansible.  It has been tested on a Mac, but should work on Linux as well.

It will build a cluster with a kraken parent and four nodes.  The nodes run a minimal [u-root](https://github.com/u-root-/u-root) image.

## Dependencies

The following need to be installed for this to work:

- [Go](https://golang.org/) - "The Go Programming Language"
- [VirtualBox](https://virtulabox.org) - "VirtualBox is a powerful x86 and AMD64/Intel64 virtualization product for enterprise as well as home use."
- [VirtualBox Extension Pack](https://www.virtualbox.org/wiki/Downloads) - "Support for USB 2.0 and USB 3.0 devices, VirtualBox RDP, disk encryption, NVMe and PXE boot for Intel cards."  We need this for PXE boot capabilities.
- [Vagrant](https://www.vagrantup.com) - "Vagrant is a tool for building and managing virtual machine environments in a single workflow."  This is used to manage our virtualbox VMs.

Note: It is no longer required to install [Ansible](https://www.ansible.com) on the host system, as ansible now runs locally within the virtual machine.

### WSL

Important: this will not work with WSL 2, since there are Vagrant/VirtualBox issues with WSL 2 (see: [this bug post](https://github.com/geerlingguy/ansible-for-devops/issues/291)).  Make sure your WSL instance is using WSL 1 (you can check with `wsl --list --verbose` in powershell).

This example can be run within Microsoft WSL (Linux on Windows).  Setup is a bit more complicated in WSL.  To set this up:

- Make sure you are using WSL 1 and WSL 2 isn't installed (it will conflict with VirtualBox).
- Make sure Go is installed on the *Linux* side.
- Make sure VirtualBox is installed on the *Windows* side.
- Make sure Vagrant is installed on the *Linux* side.
- Make sure your WSL is configured to use the "metadata" option on DrvFs. For details, see:
    - https://devblogs.microsoft.com/commandline/chmod-chown-wsl-improvements/
    - https://devblogs.microsoft.com/commandline/automatically-configuring-wsl/

    Specifically, you probably want something like the following in `/etc/wsl.conf`:
    ```ini
    [automount]
    options = "metadata"
    ```

    After configuring this option you'll need to restart your WSL instance.

When running in WSL mode, all commands should be run from the *Linux* side.

## Instructions

### Get Kraken

You need a copy of Kraken to continue.  Choose a good working directory and run:

```bash
$ git clone https://github.com/kraken-hpc/kraken
Cloning into 'kraken'...
remote: Enumerating objects: 43, done.
remote: Counting objects: 100% (43/43), done.
remote: Compressing objects: 100% (37/37), done.
remote: Total 4694 (delta 8), reused 14 (delta 3), pack-reused 4651
Receiving objects: 100% (4694/4694), 7.25 MiB | 13.00 MiB/s, done.
Resolving deltas: 100% (1742/1742), done.
$ cd kraken
```

### Setting up host-only networking

Once the dependencies have been installed, there is one step that we do not do automatically.  In the VirtualBox network settings, make sure a "host-only" network named "vboxnet99" is configured (except in WSL, see below). Since the VirtualBox GUI creates adapters in numerical order and we do not want to interfere with others predefined vboxnet entry, let's use an arbitrarily high vboxnet number. The example below is for MacOS.

Note: the host-only network should *not* have DHCP enabled.  It should also be configured to have the network address `192.168.57.1`.

#### MacOS network creation
```bash
$ /Applications/VirtualBox.app/Contents/MacOS/VBoxNetAdpCtl vboxnet99 add
$ VBoxManage hostonlyif ipconfig vboxnet99 -ip=192.168.57.1 --netmask=255.255.255.0
$ VBoxManage dhcpserver modify --netname HostInterfaceNetworking-vboxnet99 --disable
```

#### Linux network creation
```bash
$ /usr/lib/virtualbox/VBoxNetAdpCtl vboxnet99 add
$ VBoxManage hostonlyif ipconfig vboxnet99 -ip=192.168.57.1 --netmask=255.255.255.0
$ VBoxManage dhcpserver modify --netname HostInterfaceNetworking-vboxnet99 --disable
```

#### WSL network creation
Note: Modifying Host-Only networking on Windows requires Administrator access.

First, you'll need to add VirtualBox commands to your path on the Linux side (this could be a different directory depending on your install options).

```bash
$ export PATH="$PATH":"/mnt/c/Program Files/Oracle/VirtualBox"
```

The `VBoxNetAdpCtl` command does not exist in Windows VirtualBox.  You will need to create a host-only network through the GUI.  We can override the network name, but you need to set the IPv4 address to `192.168.56.1` and the IPv4 Network Mask to `255.255.255.0`. Also, ensure the `DHCP Server` box is unchecked.

In Windows we have no way to set the name for the network.  We can override the `vboxnet99` name by setting an environment variable in the shell we are working in (note: this works for other OSes as well).

We will need to specify the full name of the network addaptor.  It will be something like `VirtualBox Host-Only Ethernet Adapter #2`.  You can verify your settings and get an easy to copy string with.

```bash
$ VBoxManage.exe list hostonlyifs
Name:            VirtualBox Host-Only Ethernet Adapter #2
GUID:            5c30c01b-6970-49bf-bbd9-3b520409c641
DHCP:            Disabled
IPAddress:       192.168.57.1
NetworkMask:     255.255.255.0
IPV6Address:     fe80::786c:9254:b247:2251
IPV6NetworkMaskPrefixLength: 64
HardwareAddress: 0a:00:27:00:00:37
MediumType:      Ethernet
Wireless:        No
Status:          Up
VBoxNetworkName: HostInterfaceNetworking-VirtualBox Host-Only Ethernet Adapter #2
...
```

Now, export the variable with what ever is set in the `Name:` field:

```bash
$ export KRAKEN_VBOXNET="VirtualBox Host-Only Ethernet Adapter #2"
```

You will need to set this variable any time you create a new shell.  Alternatively, you could place it in your `.bashrc`.

## Release the Kraken

Once the dependencies are installed and the host-only network is setup in VirtualBox, you can deploy a virtual cracking cluster with one command:


```bash
$ bash release-the-kraken.sh
...
```

Note: this does *not* require root/sudo.  

If you are running under WSL, you will need to tell Windows to allow firewall access to vboxapi (Windows will pop up a message).

This script will perform all of the necessary steps to build a virtual kraken cluster.  It will take about 3-5 minutes to complete.

## How it works

The `release-the-kraken.sh` script performs the following steps to bring up a virtual Kraken cluster.

1. It verifies that we have all of the dependencies and network settings we need.
2. It calls the script `create-nodes.sh`, which further uses the `VagrantFile` to create nodes `kr[1-4]`.  `create-nodes.sh` also immediately shuts these off as we don't want them on yet.
3. Starts the (included) `vboxapi.go`, which provides a ReST API for VirtualBox VM power control.  This allows Kraken to control the power atate of the VMs.
4. It calls `vagrant up kraken`, which uses the included `VagrantFile` to create and provision the "kraken" (parent) VM.  `Vagrant` calls `ansible` to handle provisioning.  `Ansible` performs a number of steps, but here are some of the more critical ones:
   1. install necessary dependencies in the VM;
   2. get kraken, build the `kraken-builder`;
   3. build the kraken binaries;
   4. setup the directory/files needed for pxeboot;
   5. create the "layer0" image that the nodes will boot.
   6. generate node state information
   7. install and start kraken systemd service (and inject node state)
6. Loads the kraken dashboard.

From this point, Kraken takes over and brings the nodes up.  Once the nodes are up, it begins synchronizing state information with them.

To see more detail on any of these steps, take a look at the scripts and ansible roles included.

You can find more information on what's happening in the logs.  Kraken now runs under systemd, and you can reach see the logs with `journalctl -u kraken` on the "kraken" host.  To get to the kraken host you can use either:

`$ vagrant ssh kraken`

or,

`$ ssh -F ssh-config kraken`

If you want to login to one of the nodes, can get there from the "kraken" node.  They have an `ssh` server running on port `2022` and expect you to use a particular `ssh` key: `~vagrant/support/base/id_rsa`.  Here's an example to get to "kr1":

`$ ssh -p 2022 -i ~vagrant/support/base/id_rsa kr1`

## Helper scripts

There are a number of helper scripts in this directory.  Here's what they do:

- `create-nodes.sh` - this creates the `kr[1-4]` nodes.  It takes no arguments.  It skips any nodes that are already created.
- `destroy-all.sh` - this destroys everything created by `release-the-kraken.sh`.  It takes no arguments.  This includes:
   1. shutting down `vboxapi`
   a. destroying `kraken` VM
   3. destroying node VMs `kr[1-4]`
- `destroy-nodes.sh` - this destroys nodes `kr[1-4]`.  It takes no arguments.
- `release-the-kraken.sh` - this does a total bring-up of the example environment (as described above).  It takes no arguments.  This is (*should be*) safe to call more than once; you an call it multiple times to re-start kraken.
- `shutdown.sh` - this shuts down a running kraken environment by: 1) shutting down `kraken` on "kraken"; 2) shutting down the `vboxapi`. You re-start kraken by re-running `release-the-kraken.sh`.

## How to use your own fork/branch

For testing purposes, it can be nice to use your own fork & branch of kraken.  You can easily do this by changing the host vars `kr_repo` and `kr_repo_version` in `kraken.yml`.  `kr_repo` should be the full URL for the repo (e.g. https://github.com/kraken-hpc/kraken.git).  `kr_repo_version` can be any branch or tag name.  `kr_repo_version` must be provided, even if it is "main".

## Using custom kraken-build args

You can add custom arguments to `kraken-build` by setting `kr_build_args` in `kraken.yml`.  You should avoid changing `-dir` and `-config`, as the ansible role expects to set these.

The following args may be particularly helpful:

- `-pprof` - build kraken with pprof support
- `-race` - build kraken with the race detection

## How to add things to the image

Anything in the `support/base` directory will get layered into the running image.  `support/base` acts as the root (`/`) directory of the running image.  Note that there are *no* libraries (not even `glibc`) on the image by default.

Making the image large will likely lead to unexpected problems.

### Adding a kernel module

The [uinit](https://github.com/kraken-hpc/uinit) process that initializes nodes runs a utility called [modscan](https://github.com/bensallen/modscan) that will automatically add any modules needed for detected hardware.

In the case that you need to add a non-hardware module, e.g. a filesystem driver:

1. Make sure to copy the module (and any module dependencies, try `modprobe --show-depends <module>`) into the `lib/modules/$(uname -r)/` directory under `support/base`.
2. Add an entry to the uinit script to load the module at startup, e.g.:

   ```yaml
   - name: Load mymodule
     module: command
     args:
       cmd: /bbin/modprobe mymodule
   ```

## _Now: `$ go get kraken`!_
