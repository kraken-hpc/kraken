#!/bin/bash

KRAKEN_URL="github.com/hpc/kraken"

echo "==="
echo "=== Building a kraken vagrant/virtualbox cluster..."
echo "==="

GO=$(which go)
if [ $? -ne 0 ]; then
    echo "could not find go, is golang installed?"
    exit 1
fi
echo "Using go at: $GO"
 
VB=$(which vboxmanage)
if [ $? -ne 0 ]; then
    echo "could not find vboxmanage, is virtualbox installed?"
    exit 1
fi
echo "Using vboxmanage at: $VB"

VG=$(which vagrant)
if [ $? -ne 0 ]; then
    echo "could not find vagrant, is it installed?"
    exit 1
fi
echo "Using vagrant at: $VG"

AN=$(which ansible)
if [ $? -ne 0 ]; then
    echo "could not find ansible, is it installed?"
    exit 1
fi
echo "Using ansible at: $AN"

if [ -z ${GOPATH+x} ]; then
    GOPATH=$HOME/go
fi
echo "Using GOPATH: $GOPATH"

echo "Checking vbox hostonly network settings..."

${VB} list hostonlyifs | grep -E '^Name.*vboxnet99' > /dev/null 2>&1
if [ $? -ne 0 ]; then
    echo "you don't have a vboxnet99, see vbox network setup instructions"
    exit 1
fi
echo "   vboxnet99 is present"

${VB} list hostonlyifs | grep -A3 -E '^Name.*vboxnet99' | grep 192.168.57.1 > /dev/null 2>&1
if [ $? -ne 0 ]; then
    echo "vboxnet99 is not on 192.168.57.1, see vbox network setup instructions"
    exit 1
fi
echo "   vboxnet99 is on 192.168.57.1"

${VB} list hostonlyifs | grep -A2 -E '^Name.*vboxnet99' | grep -E '^DHCP.*Disabled' > /dev/null 2>&1
if [ $? -ne 0 ]; then
    echo "vboxnet99 does not have DHCP disable, see vbox network setup instructions"
    exit 1
fi
echo "   vboxnet99 DHCP is disabled"
echo "hostonly network settings OK."

echo "Creating and provisioning the master (this may take a while)..."
echo RUN: ${VG} up kraken
${VG} up kraken 2>&1 | tee -a log/vagrant-up-kraken.log

echo "Creating the compute nodes"
echo RUN: sh create-nodes.sh
sh create-nodes.sh 2>&1 | tee -a log/create-nodes.log

echo "(RE)Starting vboxapi, log file in log/vboxapi.log"
echo RUN: pkill vboxapi
pkill vboxapi
echo RUN: nohup go run $GOPATH/src/$KRAKEN_URL/utils/vboxapi/vboxapi.go -v -ip 192.168.57.1
nohup go run $GOPATH/src/$KRAKEN_URL/utils/vboxapi/vboxapi.go -v -ip 192.168.57.1 > log/vboxapi.log &


echo "(RE)Starting kraken on the 'kraken'"
echo RUN: ${VG} ssh-config kraken > ssh-config
${VG} ssh-config kraken > ssh-config
echo RUN: ssh -F ssh-config kraken 'sudo pkill kraken'
ssh -F ssh-config kraken 'sudo pkill kraken'
echo RUN: ssh -F ssh-config kraken 'sudo sh support/start-kraken.sh'
ssh -F ssh-config kraken 'sudo sh support/start-kraken.sh'


which open > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "Launching dashboard viewer to: http://192.168.57.10/"
    open http://192.168.57.10/
else 
    echo "Kraken dashboard is running on: http://192.168.57.10/"
fi

echo "Injecting kraken state/provisioning nodes"
echo RUN: sh inject-state.sh
sleep 1
sh inject-state.sh 2>&1 | tee -a log/inject-state.log

echo
echo "==="
echo "=== Done.  View the dashboard at: http://192.168.57.10/"
echo "=== Enjoy your Kraken!"
echo "==="
