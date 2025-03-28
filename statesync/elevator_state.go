package statesync

import (
	"elevator/elevio"
	"encoding/binary"
	"time"
)

type elevatorState struct {
	id            int
	nonce         int
	currFloor     int
	currDirection elevio.MotorDirection
	request       [][3]bool
	lastSync      time.Time
}

// Serializes an elevatorState into a byte slice.
func serialize(s elevatorState) []byte {
	buf := make([]byte, 0, 128)

	buf = append(buf, uint8(s.id))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(s.nonce))
	buf = append(buf, uint8(s.currFloor))
	buf = append(buf, byte(s.currDirection))

	for _, row := range s.request {
		for _, btn := range row {
			btnByte := byte(0)
			if btn {
				btnByte = 1
			}
			buf = append(buf, btnByte)
		}
	}

	return buf
}

// Deserializes a byte slice into an elevatorState.
func deserialize(m []byte) *elevatorState {
	elevatorState := &elevatorState{
		id:            int(m[0]),
		nonce:         int(binary.LittleEndian.Uint32(m[1:5])),
		currFloor:     int(m[5]),
		currDirection: elevio.MotorDirection(int8(m[6])),
		request:       make([][3]bool, 0, 128),
	}

	offset := 7
	for i := offset; i < len(m); i += 3 {
		currRow := [3]bool{m[i] == 1, m[i+1] == 1, m[i+2] == 1}
		elevatorState.request = append(elevatorState.request, currRow)
	}

	return elevatorState
}

// Gets the ID of the elevator.
func (e *elevatorState) GetID() int {
	return int(e.id)
}

// Gets the current floor of the elevator.
func (e *elevatorState) GetFloor() int {
	return int(e.currFloor)
}

// Gets the current direction of the elevator.
func (e *elevatorState) GetDirection() elevio.MotorDirection {
	return e.currDirection
}

// Gets the requests of the elevator.
func (e *elevatorState) GetRequests() [][3]bool {
	requestsCopy := make([][3]bool, len(e.request))
	for i, requests := range e.request {
		requestsCopy[i][elevio.BT_HallUp] = requests[elevio.BT_HallUp]
		requestsCopy[i][elevio.BT_HallDown] = requests[elevio.BT_HallDown]
		requestsCopy[i][elevio.BT_Cab] = requests[elevio.BT_Cab]
	}
	return requestsCopy
}
