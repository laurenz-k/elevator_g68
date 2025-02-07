package main

/*
NOTE Laurenz:
- Strategy pattern to modularize request handling: `addRequest`, `stopOnCurrentFloor`, `getNewDirection`
- Use pure function on event handlers to make code cleaner
*/

import (
	"log"
	"time"

	"elevator/elevio"
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
	elevio.Init("elevator_sim_15657:15657", numFloors)

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
	log.Printf("Pressed button %+v\n", b)

	e.addRequest(b)

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
		e.clearRequest(elevio.ButtonEvent{Floor: e.currentFoor, Button: elevio.BT_Cab})
		e.clearRequest(elevio.ButtonEvent{Floor: e.currentFoor, Button: elevio.BT_HallUp})
		e.clearRequest(elevio.ButtonEvent{Floor: e.currentFoor, Button: elevio.BT_HallDown})
	}
}

func (e *Elevator) handleFloorChange(floorNum int) {
	log.Printf("floor changed %+v\n", floorNum)

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
	log.Printf("Door obstruction %+v\n", isObstructed)

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
	prevDirection := e.direction
	e.state = ST_DoorOpen
	e.direction = elevio.MD_Stop
	elevio.SetMotorDirection(e.direction)

	elevio.SetDoorOpenLamp(true)

	e.clearFloorRequests(prevDirection)

	time.AfterFunc(e.openDoorDuration, func() {
		for e.doorObstructed {
			time.Sleep(20 * time.Millisecond)
		}
		elevio.SetDoorOpenLamp(false)

		e.setNextDirection(prevDirection)

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

func (e *Elevator) addRequest(b elevio.ButtonEvent) {
	e.requests[b.Floor][b.Button] = true
	elevio.SetButtonLamp(b.Button, b.Floor, true)
}

func (e *Elevator) clearRequest(b elevio.ButtonEvent) {
	e.requests[e.currentFoor][b.Button] = false
	elevio.SetButtonLamp(elevio.ButtonType(b.Button), b.Floor, false)
}

func (e *Elevator) clearFloorRequests(d elevio.MotorDirection) {
	// cab requests
	e.clearRequest(elevio.ButtonEvent{Floor: e.currentFoor, Button: elevio.BT_Cab})

	// same direction calls
	if d == elevio.MD_Up {
		e.clearRequest(elevio.ButtonEvent{Floor: e.currentFoor, Button: elevio.BT_HallUp})
	} else if d == elevio.MD_Down {
		e.clearRequest(elevio.ButtonEvent{Floor: e.currentFoor, Button: elevio.BT_HallDown})
	} else if d == elevio.MD_Stop {
		e.clearRequest(elevio.ButtonEvent{Floor: e.currentFoor, Button: elevio.BT_HallUp})
		e.clearRequest(elevio.ButtonEvent{Floor: e.currentFoor, Button: elevio.BT_HallDown})
	}

	// opposite direction calls iff there's no more unfilled requests in direction
	if d == elevio.MD_Up && !e.hasRequestAbove() {
		e.clearRequest(elevio.ButtonEvent{Floor: e.currentFoor, Button: elevio.BT_HallDown})
	} else if d == elevio.MD_Down && !e.hasRequestBelow() {
		e.clearRequest(elevio.ButtonEvent{Floor: e.currentFoor, Button: elevio.BT_HallUp})
	}
}

func (e *Elevator) setNextDirection(d elevio.MotorDirection) {
	// keeps same direction as long as there's requests in same direction left
	if d == elevio.MD_Up && e.hasRequestAbove() {
		e.state = ST_Moving
		e.direction = elevio.MD_Up
	} else if d == elevio.MD_Down && e.hasRequestBelow() {
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
}
