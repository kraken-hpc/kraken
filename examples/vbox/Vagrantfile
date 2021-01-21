# Get our network settings
vboxnet = ENV['KRAKEN_VBOXNET'] || 'vboxnet99'
parent_ip = ENV['KRAKEN_PARENT_IP'] || '192.168.57.10'
parent_nm = ENV['KRAKEN_PARENT_NETMASK'] || '255.255.255.0'

$script = <<-SCRIPT
echo "Installing some prerequisits to run ansible locally"
dnf -y --nogpgcheck install ansible python3-netaddr
SCRIPT

Vagrant.require_version ">= 2.0.0"
Vagrant.configure("2") do |config|
  config.vm.define "kraken" do |kraken|
    kraken.vm.hostname = "kraken"
    kraken.vm.box = "boxomatic/centos-8-stream"
    kraken.vm.network "private_network",
      ip: parent_ip,
      netmask: parent_nm, 
      virtualbox__intnet: vboxnet
    kraken.vm.network "private_network",
      ip: "10.11.12.1",
      netmask: "255.255.255.0",
      virtualbox__intnet: "intnet"
    kraken.vm.synced_folder ".", "/vagrant", type: "rsync"
    kraken.vm.synced_folder "./support", "/home/vagrant/support", type: "rsync"
    kraken.vm.provider "virtualbox" do |v|
      v.name = "kraken"
      v.linked_clone = true
      v.memory = 512
      v.cpus = 2
      v.customize [
        "modifyvm", :id,
        "--nic1", "nat",
        "--nic2", "hostonly",
        "--hostonlyadapter2", vboxnet,
        "--nic3", "intnet",
        "--intnet3", "intnet"
      ]
    end
  end

  config.vm.provision "shell", inline: $script

  config.vm.provision "ansible_local" do |ansible|
    ansible.install = false
    ansible.verbose = false
    ansible.playbook = "ansible/kraken.yml"
  end
end
