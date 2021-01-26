#!/bin/bash

echo "destroying the Kraken"

echo "stopping vboxapi"
pkill vboxapi

echo "destroying kraken"
vagrant destroy -f kraken

echo "destroying nodes"
bash destroy-nodes.sh
