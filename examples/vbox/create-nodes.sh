#!/bin/bash

# Exit on any non-zero exit code
set -o errexit

# Exit on any unset variable
set -o nounset

# Pipeline's return status is the value of the last (rightmost) command
# to exit with a non-zero status, or zero if all commands exit successfully.
set -o pipefail

for n in kr{1..4}; do 
    if vboxmanage list vms | grep -q $n; then
        echo "$n already exists, skipping"
    else
        vboxmanage createvm --name $n --ostype "RedHat_64" --register
        vboxmanage modifyvm $n \
            --nic1 intnet \
            --intnet1 intnet \
            --macaddress1 AABBCC00110${n#kr} \
            --boot1 net \
            --boot2 none \
            --boot3 none \
            --boot4 none \
            --memory 512 \
            --vram 16
    fi


done

