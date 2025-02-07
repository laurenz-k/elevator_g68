#!/bin/bash

FIRST_PORT=15657
NUM_ELEVATORS=2

for port_num in $(seq $FIRST_PORT $((FIRST_PORT+NUM_ELEVATORS-1)));
do
    cmd="docker run -it --init --rm -p $port_num:15657 --name elevator_sim_$port_num elevator_sim"

    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macos
        osascript -e "tell app \"Terminal\" to do script \"$cmd\""
        echo "Running at localhost:$port_num"

    elif [[ "$OSTYPE" == "msys" ]]; then
        # Windows
        start powershell -Command "$cmd"

    else
        echo "Unsupported OS: $OSTYPE"

    fi
done
