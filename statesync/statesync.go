package statesync

import (
	"elevator/elevio"
	"elevator/types"
	"encoding/binary"
	"log"
	"net"
	"sync"
	"time"
)

const broadcastAddr = "255.255.255.255"
const broadcastPort = "30068"
const interval = 25 * time.Millisecond
const syncTimeout = 3 * time.Second

var mtx sync.RWMutex
var states = make([]*elevatorState, 0, 10)
var thisElevatorID int
var offline bool = false

func StartStatesync(elevator types.ElevatorState, reassignmentChan chan elevio.ButtonEvent, errorChan chan string) {
	thisElevatorID = elevator.GetID()

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
}

/**
 * @brief Turns the elevator on.
 *
 * @param elevatorID The ID of the elevator to turn on.
 */
func TurnOnElevator(elevatorID int) {
	mtx.Lock()
	defer mtx.Unlock()

	if elevatorID < len(states) && states[elevatorID] != nil {
		offline = false
	}
}

/**
 * @brief Turns the elevator off.
 *
 * @param elevatorID The ID of the elevator to turn off.
 */
func TurnOffElevator(elevatorID int) {
	mtx.Lock()
	defer mtx.Unlock()

	if elevatorID < len(states) && states[elevatorID] != nil {
		offline = true
	}
}

/**
 * @brief Broadcasts the elevator's state over UDP at regular intervals.
 *
 * @param elevatorPtr The current state of the elevator to broadcast.
 */
func broadcastState(elevatorPtr types.ElevatorState) {

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		addr := broadcastAddr + ":" + broadcastPort
		conn, err := net.Dial("udp", addr)
		if err != nil {
			log.Printf("Error dialing UDP: %v", err)
			continue
		}

		defer conn.Close()

		var myState elevatorState
		nonce := 0

		for range ticker.C {
			if offline {
				continue
			}
			myState.id = elevatorPtr.GetID()
			myState.nonce = nonce
			myState.currFloor = elevatorPtr.GetFloor()
			myState.currDirection = elevatorPtr.GetDirection()
			myState.request = elevatorPtr.GetRequests()

			nonce++

			_, err = conn.Write(serialize(myState))

			if err != nil {
				break
			}
		}
	}
}

/**
 * @brief Listens for incoming elevator states over UDP and updates local states.
 */
func receiveStates() {
	addr, _ := net.ResolveUDPAddr("udp", ":"+broadcastPort)
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Printf("Error dialing UDP in reciveStates: %v", err)
		return
	}
	defer conn.Close()

	buf := make([]byte, 1024)
	for {
		n, _ := conn.Read(buf)
		stateMsg := deserialize(buf[:n])
		stateMsg.lastSync = time.Now()

		updateStates(stateMsg)
		lightHallButtons()
	}
}

/**
 * @brief Monitors elevator states and reassigns orders if an elevator is out of sync.
 */
func monitorFailedSyncs(reassignmentChan chan elevio.ButtonEvent) {
	ticker := time.NewTicker(syncTimeout)
	defer ticker.Stop()

	for range ticker.C {
		for id, s := range states {
			if s == nil {
				continue
			}
			if time.Since(s.lastSync) > syncTimeout && id != thisElevatorID {
				handleFailedSync(id, s, reassignmentChan)
			}
		}
	}
}

/**
 * @brief Handles a failed sync by reassigning orders and clearing the elevator's state.
 *
 * @param id The ID of the elevator that failed to sync.
 * @param s The state of the elevator that failed to sync.
 * @param reassignmentChan The channel to send reassigned orders to.
 */
func handleFailedSync(id int, s *elevatorState, reassignmentChan chan elevio.ButtonEvent) {
	log.Printf("Elevator %d has not synced for over %v. Reassigning orders.", id, syncTimeout)

	reassignOrders(s, reassignmentChan)
	mtx.Lock()
	states[id] = nil
	mtx.Unlock()
}

/**
 * @brief Reassigns all active orders from a failed elevator to the reassignment channel.
 *
 * @param s The state of the elevator that failed to sync.
 * @param reassignmentChan The channel to send reassigned orders to.
 */
func reassignOrders(s *elevatorState, reassignmentChan chan elevio.ButtonEvent) {
	btns := [...]elevio.ButtonType{elevio.BT_HallDown, elevio.BT_HallUp}
	for floor, order := range s.request {
		for _, btn := range btns {
			if order[btn] {
				reassignmentChan <- elevio.ButtonEvent{
					Floor:  floor,
					Button: elevio.ButtonType(btn),
				}
				log.Printf("Reassigning order from elevator %d: %d %d", s.id, floor, btn)
				s.request[floor][btn] = false
			}
		}
	}
}

/**
 * @brief Retrieves the stored state of the given elevator.
 *
 * @param elevatorID The ID of the elevator.
 * @return Pointer to the elevatorState, or nil if not found.
 */
func GetState(elevatorID int) types.ElevatorState {
	mtx.RLock()
	defer mtx.RUnlock()
	if elevatorID < len(states) {
		return states[elevatorID]
	}
	return nil
}

/**
 * @brief Gets the IDs of all elevators that have synced within the timeout.
 *
 * @return A slice of active elevator IDs.
 */
func GetAliveElevatorIDs() []int {
	mtx.RLock()
	defer mtx.RUnlock()

	alive := make([]int, 0, len(states))
	alive = append(alive, thisElevatorID)

	for id, s := range states {
		if s != nil {
			alive = append(alive, id)
		}
	}
	log.Printf("Alive elevators: %v", alive)
	return alive
}

/**
 * @brief Serializes an elevatorState into a byte slice.
 *
 * @param s The elevator state to serialize.
 * @return A byte slice representing the serialized state.
 */
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

/**
 * @brief Deserializes a byte slice into an elevatorState.
 *
 * @param m The byte slice containing serialized elevator state data.
 * @return Pointer to the deserialized elevatorState.
 */
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

/**
 * @brief Updates the stored state of an elevator.
 *
 * @param s The new state of the elevator.
 */
func updateStates(s *elevatorState) {
	mtx.Lock()
	defer mtx.Unlock()

	id := s.id
	if id >= len(states) {
		states = append(states, make([]*elevatorState, (id+1)-len(states))...)
	}

	vOld := states[id]
	if vOld == nil || vOld.nonce < s.nonce {
		states[id] = s
	}
}

// NOTE laurenz-k: maybe refactor to controller?
var prevHallButtonLights [][2]bool

/**
 * @brief Lights up the hall buttons based on the aggregated requests.
 */
func lightHallButtons() {
	buttonsToLight := orAggregateAllLiveRequests()

	for i, row := range buttonsToLight {
		for j, val := range row {
			if prevHallButtonLights == nil || prevHallButtonLights[i][j] != val {
				elevio.SetButtonLamp(elevio.ButtonType(j), i, val)
			}
		}
	}

	prevHallButtonLights = buttonsToLight
}

/**
 * @brief Aggregates all live requests from all elevators.
 *
 * @return A 2D slice representing the aggregated requests.
 */
func orAggregateAllLiveRequests() [][2]bool {
	var floorCount int
	for _, state := range states {
		if state != nil {
			floorCount = len(state.request)
			break
		}
	}

	aggMatrix := make([][2]bool, floorCount)

	for _, state := range states {
		if state == nil || time.Since(state.lastSync) > syncTimeout {
			continue
		}

		for floor_i := range len(aggMatrix) {
			for btn_i := range len(aggMatrix[floor_i]) {
				aggMatrix[floor_i][btn_i] = aggMatrix[floor_i][btn_i] || state.request[floor_i][btn_i]
			}
		}
	}

	return aggMatrix
}

// fits better with controller??
/**
 * @brief Detects if an elevator is stuck and sets its online flag accordingly.
 */
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

/**
 * @brief Updates the lastActionTime of an elevator if it changes direction or floor.
 */
func timeSinceLastAction(elevator types.ElevatorState) {
	if elevator.GetFloor() != prevFloor || elevator.GetDirection() != prevDirection {
		lastActionTime = time.Now()
		prevFloor = elevator.GetFloor()
		prevDirection = elevator.GetDirection()
	}
}
