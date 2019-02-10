# Kraken vagrant/virtualbox example

The contents of this directory can be used to generate a VirtualBox virtual kraken cluster using Vagrant and Ansible.  It has been tested on a Mac, but should work on Linux as well.

It will build a cluster with a master and four nodes.  The nodes run a minimal [u-root](https://github.com/u-root-/u-root) image.

## Dependencies

The following need to be installed for this to work:
- [VirtualBox](https://virtulabox.org) - "VirtualBox is a powerful x86 and AMD64/Intel64 virtualization product for enterprise as well as home use."
- [VirtualBox Extension Pack](https://www.virtualbox.org/wiki/Downloads) - "Support for USB 2.0 and USB 3.0 devices, VirtualBox RDP, disk encryption, NVMe and PXE boot for Intel cards."  We need this for PXE boot capabilities.
- [Vagrant](https://www.vagrantup.com) - "Vagrant is a tool for building and managing virtual machine environments in a single workflow."  This is used to manage our virtualbox VMs.
- [Ansible](https://www.ansible.com) - Orchestration/configuration management tool.  We use this to provision the master.

## Instructions

Once the dependencies have been installed, there is one step that we do not do automatically.  In the VirtualBox network settings, make sure a "host-only" network named "vboxnet1" is configured.  It should *not* have DHCP enabled.  It should also be configured to have the network address `192.168.57.1`.

Once the dependencies are installed and the host-only network is setup in VirtualBox, you can deploy a virtual cracking cluster with one command:

`$ sh release-the-kraken.sh`

Note: this does *not* require root/sudo.  

This script will perform all of the nessesary steps to build a virtual kraken cluster.  It will take about 3-5 minutes to complete.

## How it works

The `release-the-kraken.sh` script performs the following steps to bring up a virtual Kraken cluster.

1. It verifies that we have all of the dependencies and network settings we need.
2. It calls `vagrant up kraken`, which uses the included `VagrantFile` to create and provision the "kraken" (master) VM.  `Vagrant` calls `ansible` to handle provisioning.  `Ansible` performs a number of steps, but here are some of the more critical ones:
   1. install necessary dependencies in the VM;
   2. get kraken, build the `kraken-builder`;
   3. build the kraken binaries;
   4. setup the directory/files needed for pxeboot;
   5. create the "layer0" image that the nodes will boot.
3. It calls the script `create-nodes.sh`, which further uses the `VagrantFile` to create nodes `kr[1-4]`.  `create-nodes.sh` also immediatelly shuts these off as we don't want them on yet.
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

_Now, `$ go get kraken`!_