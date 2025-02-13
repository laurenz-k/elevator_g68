package statesync

import "elevator/controller"

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
