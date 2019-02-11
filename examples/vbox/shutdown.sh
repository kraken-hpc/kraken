#!/bin/bash

echo Stopping kraken
ssh -F ssh-config kraken 'sudo pkill kraken'

echo Stopping vboxapi
pkill vboxapi