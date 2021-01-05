#!/bin/bash

# Exit on any non-zero exit code
set -o errexit

# Exit on any unset variable
set -o nounset

# Pipeline's return status is the value of the last (rightmost) command
# to exit with a non-zero status, or zero if all commands exit successfully.
set -o pipefail

VBOXNET=${KRAKEN_VBOXNET:-"vboxnet99"}
VBOXNET_IP=${KRAKEN_VBOXNET_IP:-"192.168.57.1"}
KRAKEN_IP=${KRAKEN_PARENT_IP:-"192.168.57.10"}

### Functions

function test_chmod {
    touch .chmod_test
    chmod 0600 .chmod_test
    s=$(stat -c "%a" .chmod_test)
    rm -f .chmod_test
    [[ $s == 600 ]] && return 0 || return 1
}

###

echo "==="
echo "=== Building a kraken vagrant/virtualbox cluster..."
echo "=="

# Are we running under WSL?
# See: https://stackoverflow.com/questions/38086185/how-to-check-if-a-program-is-run-in-bash-on-ubuntu-on-windows-and-not-just-plain
VBCMD="vboxmanage"
if grep -qEi "(Microsoft|WSL)" /proc/version &> /dev/null ; then
    echo "Detected Microsoft WSL"
    # setup VAGRANT env vars for vagrant control inside of WSL
    export VAGRANT_WSL_WINDOWS_ACCESS_USER_HOME_PATH="$PWD"
    export VAGRANT_WSL_ENABLE_WINDOWS_ACCESS=1
    # use the vboxmanager on the host OS
    VBCMD="VBoxManage.exe"
    # can we chmod?
    if ! test_chmod ; then
        echo "cannot chmod files.  This probably means you have DrvFs mounted without the metadata option."
        echo "See: https://devblogs.microsoft.com/commandline/chmod-chown-wsl-improvements/"
	echo "and, https://devblogs.microsoft.com/commandline/automatically-configuring-wsl/"
        exit 1
    fi
fi

if ! GO=$(command -v go); then
    echo "could not find go, is golang installed?"
    exit 1
fi
echo "Using go at: $GO"
 
if ! VB=$(command -v "$VBCMD"); then
    echo "could not find vboxmanage, is virtualbox installed?"
    exit 1
fi
echo "Using vboxmanage at: $VB"

if ! VG=$(command -v vagrant); then
    echo "could not find vagrant, is it installed?"
    exit 1
fi
echo "Using vagrant at: $VG"

if ! AN=$(command -v ansible); then
    echo "could not find ansible, is it installed?"
    exit 1
fi
echo "Using ansible at: $AN"

GOPATH="${GOPATH:-"$HOME/go"}"

echo "Using GOPATH: $GOPATH"

BASEDIR=$(dirname "$PWD")
VBOXAPI="$(dirname "$BASEDIR")/utils/vboxapi/vboxapi.go"

if [ ! -f  "$VBOXAPI" ]; then
    echo "Could not find vboxapi.go"
    exit 1
fi

echo "Using vboxapi: $VBOXAPI"

echo "Checking vbox hostonly network settings..."

if ! "$VB" list hostonlyifs | grep -q -E "^Name.*${VBOXNET}"; then
    echo "you don't have a ${VBOXNET}, see vbox network setup instructions"
    exit 1
fi
echo "   ${VBOXNET} is present"

if ! "$VB" list hostonlyifs | grep -A3 -E "^Name.*${VBOXNET}" | grep -q -w "${VBOXNET_IP}"; then
    echo "${VBOXNET} is not on ${VBOXNET_IP}, see vbox network setup instructions"
    exit 1
fi
echo "   ${VBOXNET} interface is configured with ${VBOXNET_IP}"

# if we actually exit on failure, we fail if no VMs have run on the network.  Just give a warning.
if ! (ifconfig "${VBOXNET}" 2>/dev/null || ip addr show "${VBOXNET}" 2>/dev/null) | grep -q -w "${VBOXNET_IP}"; then
    echo "WARNING: ${VBOXNET} interface is not set to ${VBOXNET_IP}, this may just be because vbox hasn't initialized it yet"
else 
    echo "   ${VBOXNET} interface has IP ${VBOXNET_IP}"
fi

if ! "$VB" list hostonlyifs | grep -A2 -E "^Name.*${VBOXNET}" | grep -q -E '^DHCP.*Disabled'; then
    echo "${VBOXNET} does not have DHCP disable, see vbox network setup instructions"
    exit 1
fi
echo "   ${VBOXNET} DHCP is disabled"
echo "hostonly network settings OK."

echo "Creating and provisioning the master (this may take a while)..."
echo RUN: "${VG}" up kraken
"${VG}" up kraken 2>&1 | tee -a log/vagrant-up-kraken.log

echo "Creating the compute nodes"
echo RUN: bash create-nodes.sh
bash create-nodes.sh 2>&1 | tee -a log/create-nodes.log

echo "(RE)Starting vboxapi, log file in log/vboxapi.log"
echo RUN: pkill vboxapi
pkill vboxapi || true
echo RUN: nohup go run "${VBOXAPI}" -v -ip "${VBOXNET_IP}"
nohup go run "${VBOXAPI}" -v -ip "${VBOXNET_IP}" -vbm "${VB}" > log/vboxapi.log &

echo "(RE)Starting kraken on the 'kraken'"
echo RUN: "${VG}" ssh-config kraken > ssh-config
"${VG}" ssh-config kraken > ssh-config
echo RUN: ssh -F ssh-config kraken 'sudo pkill kraken'
ssh -F ssh-config kraken 'sudo pkill kraken' || true
echo RUN: ssh -F ssh-config kraken 'sudo sh support/start-kraken.sh'
ssh -F ssh-config kraken 'sudo sh support/start-kraken.sh'

if command -v open > /dev/null; then
    echo "Launching dashboard viewer to: http://${KRAKEN_IP}/"
    open "http://${KRAKEN_IP}/"
else 
    echo "Kraken dashboard is running on: http://${KRAKEN_IP}/"
fi

echo "Injecting kraken state/provisioning nodes"
echo RUN: bash inject-state.sh
sleep 1
bash inject-state.sh "${KRAKEN_IP}" 2>&1 | tee -a log/inject-state.log

echo
echo "==="
echo "=== Done.  View the dashboard at: http://${KRAKEN_IP}/"
echo "=== Enjoy your Kraken!"
echo "==="
