#!/bin/bash

# Exit on any non-zero exit code
set -o errexit

# Exit on any unset variable
set -o nounset

# Pipeline's return status is the value of the last (rightmost) command
# to exit with a non-zero status, or zero if all commands exit successfully.
set -o pipefail

echo Stopping kraken
ssh -F ssh-config kraken 'sudo pkill kraken' || true

echo Stopping vboxapi
pkill vboxapi || true