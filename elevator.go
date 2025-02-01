package main

import (
	"fmt"
	"time"

	elevio "elevator/elevio"
)

type ElevatorState int

const (
	ST_Idle     ElevatorState = 0
	ST_Moving   ElevatorState = 1
	ST_DoorOpen ElevatorState = 2
)

type Elevator struct {
	State            ElevatorState
	CurrentFoor      int
	Direction        elevio.MotorDirection
	Requests         [][3]bool
	OpenDoorDuration time.Duration
	DoorObstructed   bool
}

func NewElevator(numFloors int, openDoorDuration time.Duration) Elevator {
	elevio.Init("localhost:15657", numFloors)

	betweenFloors := elevio.GetFloor() == -1
	if betweenFloors {
		elevio.SetMotorDirection(elevio.MD_Up)
		for elevio.GetFloor() == -1 {
			time.Sleep(20 * time.Millisecond)
		}
		elevio.SetMotorDirection(elevio.MD_Stop)
	}

	return Elevator{
		State:            ST_Idle,
		CurrentFoor:      elevio.GetFloor(),
		Direction:        elevio.MD_Stop,
		Requests:         make([][3]bool, numFloors),
		OpenDoorDuration: openDoorDuration,
		DoorObstructed:   false,
	}
}

func (elevator *Elevator) Run() {
	drv_buttons := make(chan elevio.ButtonEvent)
	drv_floors := make(chan int)
	drv_obstr := make(chan bool)
	drv_stop := make(chan bool)

	go elevio.PollButtons(drv_buttons)
	go elevio.PollFloorSensor(drv_floors)
	go elevio.PollObstructionSwitch(drv_obstr)
	go elevio.PollStopButton(drv_stop)

	for {
		select {
		case a := <-drv_buttons:
			elevator.handleButtonPress(a)

		case a := <-drv_floors:
			elevator.handleFloorChange(a)

		case a := <-drv_obstr:
			elevator.handleDoorObstruction(a)

		case a := <-drv_stop:
			elevator.handleStopButton(a)
		}
	}
}

func (e *Elevator) handleButtonPress(b elevio.ButtonEvent) {
	fmt.Printf("Pressed button %+v\n", b)

	e.Requests[b.Floor][b.Button] = true
	elevio.SetButtonLamp(b.Button, b.Floor, true)

	switch e.State {
	case ST_Idle:
		if e.CurrentFoor < b.Floor {
			e.State = ST_Moving
			e.Direction = elevio.MD_Up
			elevio.SetMotorDirection(e.Direction)
		} else if e.CurrentFoor > b.Floor {
			e.State = ST_Moving
			e.Direction = elevio.MD_Down
			elevio.SetMotorDirection(e.Direction)
		} else {
			// TODO refactor into openDoor function
			e.State = ST_DoorOpen
			elevio.SetDoorOpenLamp(true)
			for buttonType := 0; buttonType < len(e.Requests[e.CurrentFoor]); buttonType++ {
				e.Requests[e.CurrentFoor][buttonType] = false
				elevio.SetButtonLamp(elevio.ButtonType(buttonType), e.CurrentFoor, false)
			}
			time.AfterFunc(e.OpenDoorDuration, func() {
				for e.DoorObstructed {
					time.Sleep(20 * time.Millisecond)
				}
				// getNewDirection function
				newDir := elevio.MD_Stop
				if e.requestAbove() {
					newDir = elevio.MD_Up
				} else if e.requestBelow() {
					newDir = elevio.MD_Down
				}
				// getNewDirection function

				elevio.SetDoorOpenLamp(false)
				if newDir != elevio.MD_Stop {
					e.State = ST_Moving
					e.Direction = newDir
				} else {
					e.State = ST_Idle
					e.Direction = newDir
				}
				elevio.SetMotorDirection(e.Direction)
			})
			// TODO refactor into openDoor function
		}
	case ST_Moving:
		break
	case ST_DoorOpen:
		// door is open, so any button press on current floor cleared immediately
		// TODO maybe restart the 3 sec timer here?
		for buttonType := 0; buttonType < len(e.Requests[e.CurrentFoor]); buttonType++ {
			e.Requests[e.CurrentFoor][buttonType] = false
			elevio.SetButtonLamp(elevio.ButtonType(buttonType), e.CurrentFoor, false)
		}
	}
}

func (e *Elevator) handleFloorChange(floorNum int) {
	fmt.Printf("floor changed %+v\n", floorNum)

	switch e.State {
	case ST_Moving:
		e.CurrentFoor = floorNum
		elevio.SetFloorIndicator(floorNum)

		if e.stopOnCurrentFloor() {
			// TODO refactor into openDoor function
			prevDir := e.Direction

			e.State = ST_DoorOpen
			e.Direction = elevio.MD_Stop
			elevio.SetMotorDirection(e.Direction)
			elevio.SetDoorOpenLamp(true)

			// delete cab calls
			e.Requests[e.CurrentFoor][elevio.BT_Cab] = false
			elevio.SetButtonLamp(elevio.BT_Cab, e.CurrentFoor, false)

			// delete same direction calls
			var deleteOppDirRequests bool
			if prevDir == elevio.MD_Up {
				deleteOppDirRequests = !e.Requests[e.CurrentFoor][elevio.BT_HallUp]
				e.Requests[e.CurrentFoor][elevio.BT_HallUp] = false
				elevio.SetButtonLamp(elevio.BT_HallUp, e.CurrentFoor, false)
			} else if prevDir == elevio.MD_Down {
				deleteOppDirRequests = !e.Requests[e.CurrentFoor][elevio.BT_HallDown]
				e.Requests[e.CurrentFoor][elevio.BT_HallDown] = false
				elevio.SetButtonLamp(elevio.BT_HallDown, e.CurrentFoor, false)
			}

			// TODO cab call and opposite direction call => clear opposite direction calll
			// TODO now dome intermediate calls also get deleted...

			// delete opposite direction calls if it's the last stop in this direction
			if deleteOppDirRequests { //  || e.CurrentFoor == len(e.Requests) || e.CurrentFoor == 0
				if prevDir == elevio.MD_Up {
					e.Requests[e.CurrentFoor][elevio.BT_HallDown] = false
					elevio.SetButtonLamp(elevio.BT_HallDown, e.CurrentFoor, false)
				} else if prevDir == elevio.MD_Down {
					e.Requests[e.CurrentFoor][elevio.BT_HallUp] = false
					elevio.SetButtonLamp(elevio.BT_HallUp, e.CurrentFoor, false)
				}
			}

			// dispatch elevator again
			time.AfterFunc(e.OpenDoorDuration, func() {
				for e.DoorObstructed {
					time.Sleep(20 * time.Millisecond)
				}
				elevio.SetDoorOpenLamp(false)

				// keep same direction as long as there's requests in same direction left
				if prevDir == elevio.MD_Up && e.requestAbove() {
					e.State = ST_Moving
					e.Direction = elevio.MD_Up
				} else if prevDir == elevio.MD_Up && e.requestBelow() {
					e.State = ST_Moving
					e.Direction = elevio.MD_Down
				} else if e.requestAbove() {
					e.State = ST_Moving
					e.Direction = elevio.MD_Up
				} else if e.requestBelow() {
					e.State = ST_Moving
					e.Direction = elevio.MD_Down
				} else {
					e.State = ST_Idle
					e.Direction = elevio.MD_Stop
				}
				elevio.SetMotorDirection(e.Direction)
			})
			// TODO refactor into openDoor function
		}

	case ST_Idle:
		panic("Floor changed in state \"ST_Idle\"")

	case ST_DoorOpen:
		panic("Floor changed in state \"ST_DoorOpen\"")
	}
}

func (e *Elevator) handleDoorObstruction(isObstructed bool) {
	fmt.Printf("Door obstruction %+v\n", isObstructed)

	switch e.State {
	case ST_DoorOpen:
		e.DoorObstructed = isObstructed

	case ST_Moving:
		panic("Door obstructed in state \"ST_Moving\"")

	case ST_Idle:
		panic("Door obstructed in state \"ST_Idle\"")
	}
}

func (e *Elevator) handleStopButton(isPressed bool) {
	panic("Stop button not implemented")
}

func (e *Elevator) stopOnCurrentFloor() bool {
	// take all cab requests
	if e.Requests[e.CurrentFoor][elevio.BT_Cab] {
		return true
	}

	if e.Direction == elevio.MD_Up {
		// take requests in same direction
		if e.Requests[e.CurrentFoor][elevio.BT_HallUp] {
			return true
		}
		// take requests in opposite direction iff no unanswered requests in same direction
		if e.Requests[e.CurrentFoor][elevio.BT_HallDown] && !e.requestAbove() {
			return true
		}

	} else if e.Direction == elevio.MD_Down {
		// take requests in same direction
		if e.Requests[e.CurrentFoor][elevio.BT_HallDown] {
			return true
		}
		// take requests in opposite direction iff no unanswered requests in same direction
		if e.Requests[e.CurrentFoor][elevio.BT_HallUp] && !e.requestBelow() {
			return true
		}
	}

	return false
}

func (e *Elevator) requestAbove() bool {
	for floor := e.CurrentFoor + 1; floor < len(e.Requests); floor++ {
		for buttonType := 0; buttonType < len(e.Requests[floor]); buttonType++ {
			if e.Requests[floor][buttonType] {
				return true
			}
		}
	}
	return false
}

func (e *Elevator) requestBelow() bool {
	for floor := e.CurrentFoor - 1; floor > -1; floor-- {
		for buttonType := 0; buttonType < len(e.Requests[floor]); buttonType++ {
			if e.Requests[floor][buttonType] {
				return true
			}
		}
	}
	return false
}
