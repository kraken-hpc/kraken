Vagrant.require_version ">= 2.0.0"
Vagrant.configure("2") do |config|
  config.vm.define "kraken" do |kraken|
    kraken.vm.hostname = "kraken"
    kraken.vm.box = "centos/7"
    kraken.vm.network "private_network",
      ip: "192.168.57.10",
      netmask: "255.255.255.0",
      virtualbox__intnet: "vboxnet1"
    kraken.vm.network "private_network",
      ip: "10.11.12.1",
      netmask: "255.255.255.0",
      virtualbox__intnet: "intnet"
    kraken.vm.synced_folder "./support", "/home/vagrant/support", type: "rsync"
    kraken.vm.provider "virtualbox" do |v|
      v.name = "kraken"
      v.linked_clone = true
      v.memory = 512
      v.cpus = 2
#      v.gui = true
      v.customize [
        "modifyvm", :id,
        "--nic1", "nat",
        "--nic2", "hostonly",
        "--hostonlyadapter2", "vboxnet1",
        "--nic3", "intnet",
        "--intnet3", "intnet"
      ]
    end
  end

  (1..4).each do |kr_idx|
    config.vm.define "kr#{kr_idx}", autostart: (kr_idx>0 ? false : true) do |kr|
      kr.vm.boot_timeout = 1 # we expect these to fail, make it quick
      kr.vm.box = "centos/7"
      kr.vm.hostname = "kr#{kr_idx}"
      kr.vm.provider "virtualbox" do |v|
        v.name = "kr#{kr_idx}"
        v.memory = 512
        v.cpus = 1
        v.linked_clone = true
        v.customize [
          'modifyvm', :id,
          '--nic1', 'intnet',
          '--intnet1', 'intnet',
          '--macaddress1', "AABBCC00110#{kr_idx}",
          '--boot1', 'net',
          '--boot2', 'none',
          '--boot3', 'none',
          '--boot4', 'none'
        ]
      end
    end
  end

config.vm.provision :ansible do |ansible|
  ansible.playbook = "kraken.yml"
end
end
