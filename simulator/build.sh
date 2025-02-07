#!/bin/bash

NETWORK_NAME='simulation-network'
SIMULATOR_NAME='elevator_simulator'
CONTROLLER_NAME='elevator_controller'

# Setup network
if ! docker network ls | grep -q $NETWORK_NAME; then
    echo "Creating Docker network: $NETWORK_NAME"
    docker network create $NETWORK_NAME
fi

# Build images
docker build -t $SIMULATOR_NAME .
cd ..
docker build -t $CONTROLLER_NAME .