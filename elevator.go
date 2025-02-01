package main

import (
	"fmt"
	"time"

	"elevator/elevio"

	"github.com/golang/glog"
)

type ElevatorState int

const (
	ST_Idle     ElevatorState = 0
	ST_Moving   ElevatorState = 1
	ST_DoorOpen ElevatorState = 2
)

type Elevator struct {
	state            ElevatorState
	currentFoor      int
	direction        elevio.MotorDirection
	requests         [][3]bool
	openDoorDuration time.Duration
	doorObstructed   bool
}

func NewElevator(numFloors int, openDoorDuration time.Duration) *Elevator {
	elevio.Init("localhost:15657", numFloors)

	betweenFloors := elevio.GetFloor() == -1
	if betweenFloors {
		elevio.SetMotorDirection(elevio.MD_Up)
		for elevio.GetFloor() == -1 {
			time.Sleep(20 * time.Millisecond)
		}
		elevio.SetFloorIndicator(elevio.GetFloor())
		elevio.SetMotorDirection(elevio.MD_Stop)
	}

	return &Elevator{
		state:            ST_Idle,
		currentFoor:      elevio.GetFloor(),
		direction:        elevio.MD_Stop,
		requests:         make([][3]bool, numFloors),
		openDoorDuration: openDoorDuration,
		doorObstructed:   false,
	}
}

func (elevator *Elevator) Run() {
	defer glog.Flush()

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
	glog.Info(fmt.Sprintf("Pressed button %+v\n", b))

	e.requests[b.Floor][b.Button] = true
	elevio.SetButtonLamp(b.Button, b.Floor, true)

	switch e.state {
	case ST_Idle:
		if e.currentFoor < b.Floor {
			e.state = ST_Moving
			e.direction = elevio.MD_Up
			elevio.SetMotorDirection(e.direction)
		} else if e.currentFoor > b.Floor {
			e.state = ST_Moving
			e.direction = elevio.MD_Down
			elevio.SetMotorDirection(e.direction)
		} else {
			e.openAndCloseDoor()
		}
	case ST_Moving:
		break
	case ST_DoorOpen:
		// door open => clear button immediately
		for buttonType := 0; buttonType < len(e.requests[e.currentFoor]); buttonType++ {
			e.requests[e.currentFoor][buttonType] = false
			elevio.SetButtonLamp(elevio.ButtonType(buttonType), e.currentFoor, false)
		}
	}
}

func (e *Elevator) handleFloorChange(floorNum int) {
	glog.Info(fmt.Sprintf("floor changed %+v\n", floorNum))

	switch e.state {
	case ST_Moving:
		e.currentFoor = floorNum
		elevio.SetFloorIndicator(floorNum)

		if e.stopOnCurrentFloor() {
			e.openAndCloseDoor()
		}

	case ST_Idle:
		panic("Floor changed in state \"ST_Idle\"")

	case ST_DoorOpen:
		panic("Floor changed in state \"ST_DoorOpen\"")
	}
}

func (e *Elevator) handleDoorObstruction(isObstructed bool) {
	glog.Info("Door obstruction %+v\n", isObstructed)

	switch e.state {
	case ST_DoorOpen:
		e.doorObstructed = isObstructed

	case ST_Moving:
		panic("Door obstructed in state \"ST_Moving\"")

	case ST_Idle:
		panic("Door obstructed in state \"ST_Idle\"")
	}
}

func (e *Elevator) handleStopButton(isPressed bool) {
	panic("Stop button not implemented")
}

func (e *Elevator) openAndCloseDoor() {
	prevDir := e.direction
	e.state = ST_DoorOpen
	e.direction = elevio.MD_Stop
	elevio.SetMotorDirection(e.direction)

	elevio.SetDoorOpenLamp(true)

	// delete cab requests
	e.requests[e.currentFoor][elevio.BT_Cab] = false
	elevio.SetButtonLamp(elevio.BT_Cab, e.currentFoor, false)

	// delete same direction calls
	if prevDir == elevio.MD_Up {
		e.requests[e.currentFoor][elevio.BT_HallUp] = false
		elevio.SetButtonLamp(elevio.BT_HallUp, e.currentFoor, false)
	} else if prevDir == elevio.MD_Down {
		e.requests[e.currentFoor][elevio.BT_HallDown] = false
		elevio.SetButtonLamp(elevio.BT_HallDown, e.currentFoor, false)
	} else if prevDir == elevio.MD_Stop {
		e.requests[e.currentFoor][elevio.BT_HallUp] = false
		e.requests[e.currentFoor][elevio.BT_HallDown] = false
		elevio.SetButtonLamp(elevio.BT_HallUp, e.currentFoor, false)
		elevio.SetButtonLamp(elevio.BT_HallDown, e.currentFoor, false)
	}

	// delete opposite direction calls iff there's no more unfilled requests in direction
	if prevDir == elevio.MD_Up && !e.hasRequestAbove() {
		e.requests[e.currentFoor][elevio.BT_HallDown] = false
		elevio.SetButtonLamp(elevio.BT_HallDown, e.currentFoor, false)
	} else if prevDir == elevio.MD_Down && !e.hasRequestBelow() {
		e.requests[e.currentFoor][elevio.BT_HallUp] = false
		elevio.SetButtonLamp(elevio.BT_HallUp, e.currentFoor, false)
	}

	time.AfterFunc(e.openDoorDuration, func() {
		for e.doorObstructed {
			time.Sleep(20 * time.Millisecond)
		}
		elevio.SetDoorOpenLamp(false)

		// keep same direction as long as there's requests in same direction left
		if prevDir == elevio.MD_Up && e.hasRequestAbove() {
			e.state = ST_Moving
			e.direction = elevio.MD_Up
		} else if prevDir == elevio.MD_Down && e.hasRequestBelow() {
			e.state = ST_Moving
			e.direction = elevio.MD_Down
		} else if e.hasRequestAbove() {
			e.state = ST_Moving
			e.direction = elevio.MD_Up
		} else if e.hasRequestBelow() {
			e.state = ST_Moving
			e.direction = elevio.MD_Down
		} else {
			e.state = ST_Idle
			e.direction = elevio.MD_Stop
		}
		elevio.SetMotorDirection(e.direction)
	})
}

func (e *Elevator) stopOnCurrentFloor() bool {
	// take all cab requests
	if e.requests[e.currentFoor][elevio.BT_Cab] {
		return true
	}

	if e.direction == elevio.MD_Up {
		// take requests in same direction
		if e.requests[e.currentFoor][elevio.BT_HallUp] {
			return true
		}
		// take requests in opposite direction iff no unanswered requests in same direction
		if e.requests[e.currentFoor][elevio.BT_HallDown] && !e.hasRequestAbove() {
			return true
		}

	} else if e.direction == elevio.MD_Down {
		// take requests in same direction
		if e.requests[e.currentFoor][elevio.BT_HallDown] {
			return true
		}
		// take requests in opposite direction iff no unanswered requests in same direction
		if e.requests[e.currentFoor][elevio.BT_HallUp] && !e.hasRequestBelow() {
			return true
		}
	}

	return false
}

func (e *Elevator) hasRequestAbove() bool {
	for floor := e.currentFoor + 1; floor < len(e.requests); floor++ {
		for buttonType := 0; buttonType < len(e.requests[floor]); buttonType++ {
			if e.requests[floor][buttonType] {
				return true
			}
		}
	}
	return false
}

func (e *Elevator) hasRequestBelow() bool {
	for floor := e.currentFoor - 1; floor > -1; floor-- {
		for buttonType := 0; buttonType < len(e.requests[floor]); buttonType++ {
			if e.requests[floor][buttonType] {
				return true
			}
		}
	}
	return false
}
