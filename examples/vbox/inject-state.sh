#!/bin/bash

# Exit on any non-zero exit code
set -o errexit

# Exit on any unset variable
set -o nounset

# Pipeline's return status is the value of the last (rightmost) command
# to exit with a non-zero status, or zero if all commands exit successfully.
set -o pipefail

KRAKEN_IP=${1:-"192.168.57.10"}
KRAKEN_PORT=${2:-"3141"}

# start microservices
curl -X PUT -H "Content-type: application/json" -d @support/state/services-on.json "http://${KRAKEN_IP}:${KRAKEN_PORT}/cfg/nodes"

sleep 1

# inject node state
curl -X POST -H "Content-type: application/json" -d @support/state/kr1-4.json "http://${KRAKEN_IP}:${KRAKEN_PORT}/cfg/nodes"
