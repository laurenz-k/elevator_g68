package statesync

import (
	"elevator/elevio"
	"elevator/types"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

const broadcastAddr = "255.255.255.255"
const broadcastPort = "49234"
const interval = 25 * time.Millisecond
const syncTimeout = 3 * time.Second

var _initialized bool
var _mtx sync.RWMutex
var _states = make([]*elevatorState, 0, 10)
var _elevatorID int
var _heartbeatDisabled bool = false

// Init start continuously broadcasting the state of the initialized elevator and receiving
// states of other elevators and maintains a set of alive elevators.
// Use `GetAliveElevatorIDs` and `GetState` to obtain live elevators.
func Init(elevator types.ElevatorState, reassignmentChan chan elevio.ButtonEvent, errorChan chan string) {
	if _initialized {
		fmt.Println("assigner already initialized!")
		return
	}

	_elevatorID = elevator.GetID()

	go func() { //Check every second
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			ElevatorStuck(elevator, errorChan)
		}
	}()

	go broadcastState(elevator)
	go receiveStates()
	go monitorFailedSyncs(reassignmentChan)

	_initialized = true
}

// EnableHeartbeat starts sending state messages to other elevators.
func EnableHeartbeat() {
	_mtx.Lock()
	defer _mtx.Unlock()

	if _elevatorID < len(_states) && _states[_elevatorID] != nil {
		_heartbeatDisabled = false
	}
}

// DisableHeartbeat stops sending state messages to other elevators.
func DisableHeartbeat() {
	_mtx.Lock()
	defer _mtx.Unlock()

	if _elevatorID < len(_states) && _states[_elevatorID] != nil {
		_heartbeatDisabled = true
	}
}

// GetState of the elevator with `elevatorID`. Retruns nil if there's no up to date information.
func GetState(elevatorID int) types.ElevatorState {
	_mtx.RLock()
	defer _mtx.RUnlock()
	if elevatorID < len(_states) {
		return _states[elevatorID]
	}
	return nil
}

// GetAliveElevatorIDs returns a slice of IDs of all elevators which have synced within the timeout.
func GetAliveElevatorIDs() []int {
	_mtx.RLock()
	defer _mtx.RUnlock()

	alive := make([]int, 0, len(_states))

	for id, s := range _states {
		if s != nil || id == _elevatorID {
			alive = append(alive, id)
		}
	}
	log.Printf("Alive elevators: %v", alive)
	return alive
}

// Or aggregates requests from all live elevators and `myRequests`.
func GetOrAggregatedLiveRequests(myRequests [][3]bool) [][3]bool {
	aggMatrix := make([][3]bool, len(myRequests))
	copy(aggMatrix, myRequests)

	for _, state := range _states {
		if state == nil {
			continue
		}

		for floor_i := range len(aggMatrix) {
			aggMatrix[floor_i][elevio.BT_HallDown] = aggMatrix[floor_i][elevio.BT_HallDown] || state.request[floor_i][elevio.BT_HallDown]
			aggMatrix[floor_i][elevio.BT_HallUp] = aggMatrix[floor_i][elevio.BT_HallUp] || state.request[floor_i][elevio.BT_HallUp]
		}
	}

	return aggMatrix
}

// Broadcasts the elevator's state over UDP at regular intervals.
func broadcastState(elevatorPtr types.ElevatorState) {
	var conn net.Conn

	for {
		var err error
		addr := broadcastAddr + ":" + broadcastPort
		conn, err = net.Dial("udp", addr)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	defer conn.Close()

	var myState elevatorState
	nonce := 0

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if _heartbeatDisabled {
			continue
		}
		myState.id = elevatorPtr.GetID()
		myState.nonce = nonce
		myState.currFloor = elevatorPtr.GetFloor()
		myState.currDirection = elevatorPtr.GetDirection()
		myState.request = elevatorPtr.GetRequests()

		nonce++

		conn.Write(serialize(myState))
	}

}

// Listens for incoming elevator states over UDP and updates local states.
func receiveStates() {
	var conn *net.UDPConn

	for {
		var err error
		addr, _ := net.ResolveUDPAddr("udp", broadcastAddr+":"+broadcastPort)
		conn, err = net.ListenUDP("udp", addr)

		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	defer conn.Close()

	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			continue
		}
		stateMsg := deserialize(buf[:n])
		stateMsg.lastSync = time.Now()

		updateStates(stateMsg)
	}
}

// Monitors elevator states and reassigns orders if an elevator is out of sync.
func monitorFailedSyncs(reassignmentChan chan elevio.ButtonEvent) {
	ticker := time.NewTicker(syncTimeout)
	defer ticker.Stop()

	for range ticker.C {
		for id, s := range _states {
			if s == nil {
				continue
			}
			if time.Since(s.lastSync) > syncTimeout && id != _elevatorID {
				log.Printf("Elevator %d has not synced for over %v. Reassigning orders.", id, syncTimeout)

				_mtx.Lock()
				_states[id] = nil
				_mtx.Unlock()

				reassignOrders(s.request, reassignmentChan)
			}
		}
	}
}

// reassignOrders detects an elevators (`id`) which failed to sync and reasigns it's orders.
func reassignOrders(orders [][3]bool, reassignmentChan chan elevio.ButtonEvent) {
	btns := [...]elevio.ButtonType{elevio.BT_HallDown, elevio.BT_HallUp}
	for floor, order := range orders {
		for _, btn := range btns {
			if order[btn] {
				reassignmentChan <- elevio.ButtonEvent{
					Floor:  floor,
					Button: elevio.ButtonType(btn),
				}
				orders[floor][btn] = false
			}
		}
	}
}

// Updates the stored state of an elevator `s`.
func updateStates(s *elevatorState) {
	_mtx.Lock()
	defer _mtx.Unlock()

	id := s.id
	if id >= len(_states) {
		_states = append(_states, make([]*elevatorState, (id+1)-len(_states))...)
	}

	vOld := _states[id]
	if vOld == nil || vOld.nonce < s.nonce {
		_states[id] = s
	}
}

// Detects if an elevator is stuck and sets its online flag accordingly.
func ElevatorStuck(elevator types.ElevatorState, errorChan chan string) {
	timeSinceLastAction(elevator)
	hasActiveCalls := false
	requests := elevator.GetRequests()
	for _, floorRequests := range requests {
		for _, active := range floorRequests {
			if active {
				hasActiveCalls = true
				break
			}
		}
		if hasActiveCalls {
			break
		}
	}
	if (hasActiveCalls) && (time.Since(lastActionTime) > 5*time.Second && !(elevator.GetDirection() == elevio.MD_Stop)) {
		log.Printf("Elevator stuck with active calls and last action time %v", time.Since(lastActionTime))
		errorChan <- "Elevator stuck"
	}
}

// Move to a better place later
var lastActionTime time.Time
var prevFloor int
var prevDirection elevio.MotorDirection

// Updates the lastActionTime of an elevator if it changes direction or floor.
func timeSinceLastAction(elevator types.ElevatorState) {
	if elevator.GetFloor() != prevFloor || elevator.GetDirection() != prevDirection {
		lastActionTime = time.Now()
		prevFloor = elevator.GetFloor()
		prevDirection = elevator.GetDirection()
	}
}
