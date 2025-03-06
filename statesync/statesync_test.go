package statesync

import (
	"elevator/elevio"
	"reflect"
	"testing"
	"time"
)

func TestSerializeDeserialize(t *testing.T) {
	inputs := []elevatorState{
		{
			id:            0,
			nonce:         0,
			currFloor:     0,
			currDirection: elevio.MD_Up,
			request:       [][3]bool{{true, false}, {false, false}, {true, false}},
		},
		{
			id:            1,
			nonce:         0,
			currFloor:     1,
			currDirection: elevio.MD_Down,
			request:       [][3]bool{{true, false}, {false, false}, {true, false}},
		},
		{
			id:            2,
			nonce:         256,
			currFloor:     2,
			currDirection: elevio.MD_Stop,
			request:       [][3]bool{{false, false}, {false, false}},
		},
		{
			id:            9,
			nonce:         600,
			currFloor:     5,
			currDirection: elevio.MD_Stop,
			request:       [][3]bool{{true, true}},
		},
	}

	for _, input := range inputs {
		serialized := serialize(input)
		deserialized := *deserialize(serialized)

		if !reflect.DeepEqual(input, deserialized) {
			t.Errorf("Deserialized `elevatorState` does not match original.\nOriginal: %+v\nDeserialized: %+v", input, deserialized)
		}
	}
}

func TestUpdateStates(t *testing.T) {
	initState := elevatorState{
		id:            2,
		nonce:         0,
		currFloor:     4,
		currDirection: elevio.MD_Down,
		request:       [][3]bool{{true, false}, {false, false}},
		lastSync:      time.Now(),
	}
	endState := elevatorState{
		id:            2,
		nonce:         5,
		currFloor:     5,
		currDirection: elevio.MD_Down,
		request:       [][3]bool{{true, false}, {false, false}},
		lastSync:      time.Now(),
	}
	staleState := elevatorState{
		id:            2,
		nonce:         2, // old nonce not applied
		currFloor:     0,
		currDirection: elevio.MD_Down,
		request:       [][3]bool{{true, false}, {false, false}},
		lastSync:      time.Now(),
	}

	updateStates(&initState)
	updateStates(&endState)
	updateStates(&staleState)

	if !reflect.DeepEqual(*states[2], endState) {
		t.Errorf("Invalid state after applying updates")
	}
}

func TestDynamicSizingOfUpdateStates(t *testing.T) {
	// ensure arbitrary amount of elevators can join the network
	for i := range 250 {
		updateStates(&elevatorState{
			id:            uint8(i),
			nonce:         0,
			currFloor:     4,
			currDirection: elevio.MD_Down,
			request:       [][3]bool{{true, false}, {false, false}},
			lastSync:      time.Now(),
		})
	}

	for i, el := range states {
		if uint8(i) != el.id {
			t.Errorf("Invalid state after applying updates")
		}
	}
}

func TestOrAggregateAllLiveRequests(t *testing.T) {
	updateStates(&elevatorState{
		id:            1,
		nonce:         0,
		currFloor:     4,
		currDirection: elevio.MD_Down,
		request:       [][3]bool{{false, true}, {false, false}},
		lastSync:      time.Now(),
	})
	updateStates(&elevatorState{
		id:            2,
		nonce:         0,
		currFloor:     4,
		currDirection: elevio.MD_Down,
		request:       [][3]bool{{false, false}, {false, true}},
		lastSync:      time.Now(),
	})
	updateStates(&elevatorState{
		id:            3,
		nonce:         0,
		currFloor:     4,
		currDirection: elevio.MD_Down,
		request:       [][3]bool{{true, false}, {false, true}},
		lastSync:      time.Now(),
	})

	// nonlive alevator state ignored
	updateStates(&elevatorState{
		id:            2,
		nonce:         0,
		currFloor:     4,
		currDirection: elevio.MD_Down,
		request:       [][3]bool{{true, true}, {true, true}},
		lastSync:      time.Now().Add(-1 * time.Hour),
	})

	expects := [][]bool{{true, true}, {false, true}}
	is := orAggregateAllLiveRequests()
	reflect.DeepEqual(is, expects)
}
