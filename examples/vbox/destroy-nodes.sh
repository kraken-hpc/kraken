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
        echo powering off $n
        vboxmanage controlvm $n poweroff || true
        echo destroying $n
        vboxmanage unregistervm $n --delete
    else
        echo "$n doesn't exist"
    fi
done
