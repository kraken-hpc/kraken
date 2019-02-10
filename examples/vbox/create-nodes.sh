#!/bin/bash

for n in kr{1..4}; do 
    vboxmanage list vms | grep $n > /dev/null
    if [ $? -eq 0 ]; then
        echo "$n already exists, skipping"
    else
        echo "creating $n"
        vagrant up $n 2> /dev/null
        vagrant halt $n
    fi
done