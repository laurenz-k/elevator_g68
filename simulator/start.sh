#!/bin/bash

NUM_ELEVATORS=3
NETWORK_NAME='simulation-network'
SIMULATOR_NAME='elevator_simulator'
CONTROLLER_NAME='elevator_controller'

# Start elevators
for id in $(seq 1 $((NUM_ELEVATORS)));
do
    cmd_sim="docker run -it --init --rm --name ${SIMULATOR_NAME}_$id --network $NETWORK_NAME $SIMULATOR_NAME"
    cmd_controller="docker run --rm --network $NETWORK_NAME --name ${CONTROLLER_NAME}_$id $CONTROLLER_NAME -addr ${SIMULATOR_NAME}_$id:15657 -id $id"

    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macos
        osascript -e "tell app \"Terminal\" to do script \"$cmd_sim\""
        sleep 0.2s
        osascript -e "tell app \"Terminal\" to do script \"$cmd_controller\""

    elif [[ "$OSTYPE" == "msys" ]]; then
        # Windows
        start powershell -Command "$cmd_sim"
        sleep 0.2s
        start powershell -Command "$cmd_controller"

    else
        echo "Unsupported OS: $OSTYPE"

    fi
done
