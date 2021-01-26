#!/bin/bash

# Exit on any non-zero exit code
set -o errexit

# Exit on any unset variable
set -o nounset

# Pipeline's return status is the value of the last (rightmost) command
# to exit with a non-zero status, or zero if all commands exit successfully.
set -o pipefail

VBCMD="vboxmanage"
if grep -qEi "(Microsoft|WSL)" /proc/version &> /dev/null ; then
    echo "Detected Microsoft WSL"
    VBCMD="VBoxManage.exe"
fi

if ! VB=$(command -v "$VBCMD"); then
    echo "could not find vboxmanage, is virtualbox installed?"
    exit 1
fi
echo "Using vboxmanage at: $VB"

for n in kr{1..4}; do
    if "$VB" list vms | grep -q $n; then
        echo powering off $n
        "$VB" controlvm $n poweroff || true
        echo destroying $n
        "$VB" unregistervm $n --delete
    else
        echo "$n doesn't exist"
    fi
done
