package statesync

import (
	"elevator/controller"
	"elevator/elevio"
)

type elevatorState struct {
	id            uint8
	nonce         uint32
	currFloor     uint8
	currDirection elevio.MotorDirection
	request       [][2]bool
}

var states = make([][]elevatorState, 0, 10)

func BroadcastState(elevatorPtr controller.Elevator) {
	// TODO Laurenz
	// serialize elevator state and broadcast out
}

func ReceiveStates() {
	// TODO Laurenz
}

func MonitorFailedSyncs() {
	// TODO Hlynur
}

func GetState(elevatorID int) {
	// TODO Hlynur
}

func GetAliveElevatorIDs() []int {
	// TODO Hlynur
	return make([]int, 0, 5)
}
