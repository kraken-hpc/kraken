# Kraken Vagrant/Virtualbox Example

## Table of Contents

- [Kraken vagrant/virtualbox example](#kraken-vagrantvirtualbox-example)
  - [Dependencies](#dependencies)
  - [Instructions](#instructions)
  - [How it works](#how-it-works)
  - [Helper scripts](#helper-scripts)
  - [How to use your own fork/branch](#how-to-use-your-own-forkbranch)
  - [Using custom kraken-build args](#using-custom-kraken-build-args)
  - [How to add things to the image](#how-to-add-things-to-the-image)
    - [Adding a kernel module](#adding-a-kernel-module)
- [*Now: $ go get kraken!*](#now--go-get-kraken)

The contents of this directory can be used to generate a VirtualBox virtual Kraken cluster using Vagrant and Ansible.  It has been tested on a Mac, but should work on Linux as well.

It will build a cluster with a master and four nodes.  The nodes run a minimal [u-root](https://github.com/u-root-/u-root) image.

## Dependencies

The following need to be installed for this to work:

- [Go](https://golang.org/) - "The Go Programming Language"
- [VirtualBox](https://virtulabox.org) - "VirtualBox is a powerful x86 and AMD64/Intel64 virtualization product for enterprise as well as home use."
- [VirtualBox Extension Pack](https://www.virtualbox.org/wiki/Downloads) - "Support for USB 2.0 and USB 3.0 devices, VirtualBox RDP, disk encryption, NVMe and PXE boot for Intel cards."  We need this for PXE boot capabilities.
- [Vagrant](https://www.vagrantup.com) - "Vagrant is a tool for building and managing virtual machine environments in a single workflow."  This is used to manage our virtualbox VMs.
- [Ansible](https://www.ansible.com) - Orchestration/configuration management tool.  We use this to provision the master.

## Instructions

If you have not set a `$GOPATH` variable set it to where you want source code to live.  The default is `$HOME/go`.
Clone the repository into the following directory: `$GOPATH/src/github.com/hpc/`.

To setup the environment:

```bash
$ export GOPATH=$HOME/go
$ go get github.com/hpc/kraken
$ cd $GOPATH/src/github.com/hpc/kraken/examples/vbox
```

Once the dependencies have been installed, there is one step that we do not do automatically.  In the VirtualBox network settings, make sure a "host-only" network named "vboxnet99" is configured. Since the VirtualBox GUI creates adapters in numerical order and we do not want to interfere with others predefined vboxnet entry, let's use an arbitrarily high vboxnet number. The example below is for MacOS.

MacOS:
```bash
$ /Applications/VirtualBox.app/Contents/MacOS/VBoxNetAdpCtl vboxnet99 add
$ VBoxManage hostonlyif ipconfig vboxnet99 -ip=192.168.57.1 --netmask=255.255.255.0
$ VBoxManage dhcpserver modify --netname HostInterfaceNetworking-vboxnet99 --disable
```
Linux:
```bash
$ /usr/lib/virtualbox/VBoxNetAdpCtl vboxnet99 add
$ VBoxManage hostonlyif ipconfig vboxnet99 -ip=192.168.57.1 --netmask=255.255.255.0
$ VBoxManage dhcpserver modify --netname HostInterfaceNetworking-vboxnet99 --disable
```


vboxnet99 should *not* have DHCP enabled.  It should also be configured to have the network address `192.168.57.1`.

Once the dependencies are installed and the host-only network is setup in VirtualBox, you can deploy a virtual cracking cluster with one command:

```bash
$ sh release-the-kraken.sh
```

Note: this does *not* require root/sudo.  

This script will perform all of the necessary steps to build a virtual kraken cluster.  It will take about 3-5 minutes to complete.

## How it works

The `release-the-kraken.sh` script performs the following steps to bring up a virtual Kraken cluster.

1. It verifies that we have all of the dependencies and network settings we need.
2. It calls `vagrant up kraken`, which uses the included `VagrantFile` to create and provision the "kraken" (master) VM.  `Vagrant` calls `ansible` to handle provisioning.  `Ansible` performs a number of steps, but here are some of the more critical ones:
   1. install necessary dependencies in the VM;
   2. get kraken, build the `kraken-builder`;
   3. build the kraken binaries;
   4. setup the directory/files needed for pxeboot;
   5. create the "layer0" image that the nodes will boot.
3. It calls the script `create-nodes.sh`, which further uses the `VagrantFile` to create nodes `kr[1-4]`.  `create-nodes.sh` also immediately shuts these off as we don't want them on yet.
4. Starts the (included) `vboxapi.go`, which provides a ReST API for VirtualBox VM power control.  This allows Kraken to control the power state of the VMs.
5. Starts `kraken` on the "kraken" (master) node.
6. Loads the state information for the nodes.

From this point, Kraken takes over and brings the nodes up.  Once the nodes are up, it begins synchronizing state information with them.

To see more detail on any of these steps, take a look at the scripts and ansible roles included.

You can find more information on what's happening in the logs under `./log/`, or the `/home/vagrant/kraken.log` file on the "kraken" host for the logs of kraken itself.  To get to the kraken host you can use either:

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
   2. destroying `kraken` VM
   3. destroying node VMs `kr[1-4]`
- `destroy-nodes.sh` - this destroys nodes `kr[1-4]`.  It takes no arguments.
- `inject-state.sh` - this injects state information into the running kraken to get things going.  It takes the Kraken IP and port as arguments. These default to 192.168.57.1 and 3141 if not specified.
- `release-the-kraken.sh` - this does a total bring-up of the example environment (as described above).  It takes no arguments.  This is (*should be*) safe to call more than once; you an call it multiple times to re-start kraken.
- `shutdown.sh` - this shuts down a running kraken environment by: 1) shutting down `kraken` on "kraken"; 2) shutting down the `vboxapi`. You re-start kraken by re-running `release-the-kraken.sh`.

## How to use your own fork/branch

For testing purposes, it can be nice to use your own fork & branch of kraken.  You can easily do this by changing the host vars `kr_repo` and `kr_repo_version` in `kraken.yml`.  `kr_repo` should be the full URL for the repo (e.g. https://github.com/hpc/kraken.git).  `kr_repo_version` can be any branch or tag name.  `kr_repo_version` must be provided, even if it is "master".

## Using custom kraken-build args

You can add custom arguments to `kraken-build` by setting `kr_build_args` in `kraken.yml`.  You should avoid changing `-dir` and `-config`, as the ansible role expects to set these.

The following args may be particularly helpful:

- `-pprof` - build kraken with pprof support
- `-race` - build kraken with the race detection

## How to add things to the image

Anything in the `support/base` directory will get layered into the running image.  `support/base` acts as the root (`/`) directory of the running image.  Note that there are *no* libraries (not even `glibc`) on the image by default.

Making the image large will likely lead to unexpected problems.

### Adding a kernel module

There is a special file, `modules.txt`, in the image.  Any module listed there will get `insmod`-ed at start.  It does not know about module dependencies, so modules need to be listed with all dependencies and in dependency order.  

You will need to add the module files themselves to `support/base` so they can be found.  It is fine to put them anywhere in the filesystem as long as `modules.txt` has a full path to the module file.

The `insmod` in `u-root` does not currently support compressed modules.  Make sure to uncompress modules before adding them to `support/base`.

## _Now: `$ go get kraken`!_
