package statesync

import (
	"elevator/elevio"
	"reflect"
	"testing"
)

func TestSerializeDeserialize(t *testing.T) {
	inputs := []elevatorState{
		{
			id:            0,
			nonce:         0,
			currFloor:     0,
			currDirection: elevio.MD_Up,
			request:       [][2]bool{{true, false}, {false, false}, {true, false}},
		},
		{
			id:            1,
			nonce:         0,
			currFloor:     1,
			currDirection: elevio.MD_Down,
			request:       [][2]bool{{true, false}, {false, false}, {true, false}},
		},
		{
			id:            2,
			nonce:         256,
			currFloor:     2,
			currDirection: elevio.MD_Stop,
			request:       [][2]bool{{false, false}, {false, false}},
		},
		{
			id:            9,
			nonce:         600,
			currFloor:     5,
			currDirection: elevio.MD_Stop,
			request:       [][2]bool{{true, true}},
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
