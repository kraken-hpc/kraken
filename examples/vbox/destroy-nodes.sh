#!/bin/bash

for n in kr{1..4}; do 
    vboxmanage list vms | grep $n
    if [ $? -eq 0 ]; then
        echo destroying $n
        vagrant destroy -f $n
    else
        echo "$n doesn't exist"
    fi
done