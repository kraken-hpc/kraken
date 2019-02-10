#!/bin/bash

# start microservices
curl -X PUT -H "Content-type: application/json" -d @support/state/services-on.json http://192.168.57.10:3141/cfg/nodes

sleep 1
# inject node state
curl -X POST -H "Content-type: application/json" -d @support/state/kr1-4.json http://192.168.57.10:3141/cfg/nodes
