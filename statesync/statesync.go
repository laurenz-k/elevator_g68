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

// TODO: Add a way to re-sync an elevator if it has been offline or out of sync for a while

const broadcastAddr = "255.255.255.255"
const broadcastPort = "15001"
const interval = 100 * time.Millisecond
const syncTimeout = 1 * time.Second

var mtx sync.RWMutex

// We store states by elevator id (index in the slice).
var states = make([]*elevatorState, 0, 10)

// ButtonPressChan is used to notify the controller to reassign orders
// when an elevator is detected as failed.
// var ButtonPressChan chan elevio.ButtonEvent

type elevatorState struct {
	id            uint8
	nonce         uint32
	currFloor     uint8
	currDirection elevio.MotorDirection
	request       [][3]bool
	lastSync      time.Time
}

/**
 * @brief Broadcasts the elevator's state over UDP at regular intervals.
 *
 * @param elevatorPtr The current state of the elevator to broadcast.
 */
func BroadcastState(elevatorPtr types.ElevatorState) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	addr := broadcastAddr + ":" + broadcastPort
	conn, _ := net.Dial("udp", addr)
	defer conn.Close()

	var myState elevatorState
	nonce := uint32(0)

	for range ticker.C {
		myState.id = uint8(elevatorPtr.GetID())
		myState.nonce = nonce
		myState.currFloor = uint8(elevatorPtr.GetFloor())
		myState.currDirection = elevatorPtr.GetDirection()
		myState.request = elevatorPtr.GetRequests()

		nonce++

		conn.Write(serialize(myState))
	}
}

/**
 * @brief Listens for incoming elevator states over UDP and updates local states.
 */
func ReceiveStates() {
	addr, _ := net.ResolveUDPAddr("udp", ":"+broadcastPort)
	conn, _ := net.ListenUDP("udp", addr)
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
func MonitorFailedSyncs() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		mtx.Lock()
		for id, s := range states {
			if s == nil {
				continue
			}
			if time.Since(s.lastSync) > syncTimeout {
				log.Printf("Elevator %d has not synced for over %v. Reassigning orders.", id, syncTimeout)
				/*for floor, order := range s.request {
					for btn, active := range order {
						if active {
							event := elevio.ButtonEvent{
								Floor:  floor,
								Button: elevio.ButtonType(btn),
							}
							if ButtonPressChan != nil {
								ButtonPressChan <- event
							} else {
								log.Printf("ButtonPressChan is nil; cannot reassign order: %+v", event)
							}
							// Clear the order after reassigning.
							s.request[floor][btn] = false
						}
					}
				}*/
			}
		}
		mtx.Unlock()
	}
}

/**
 * @brief Retrieves the stored state of the given elevator.
 *
 * @param elevatorID The ID of the elevator.
 * @return Pointer to the elevatorState, or nil if not found.
 */
func GetState(elevatorID int) *elevatorState {
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
	for id, s := range states {
		if s != nil && time.Since(s.lastSync) <= syncTimeout {
			alive = append(alive, id)
		}
	}
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

	buf = append(buf, s.id)
	buf = binary.LittleEndian.AppendUint32(buf, s.nonce)
	buf = append(buf, s.currFloor)
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
		id:            m[0],
		nonce:         binary.LittleEndian.Uint32(m[1:5]),
		currFloor:     m[5],
		currDirection: elevio.MotorDirection(int8(m[6])),
		request:       make([][3]bool, 0, 128),
	}

	offset := 7
	for i := offset; i < len(m); i += 3 {
		currRow := [...]bool{m[i] == 1, m[i+1] == 1, m[i+2] == 1}
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
	if id >= uint8(len(states)) {
		states = append(states, make([]*elevatorState, (id+1)-uint8(len(states)))...)
	}

	vOld := states[id]
	if vOld == nil || vOld.nonce < s.nonce {
		states[id] = s
	}
}

func lightHallButtons() {
	buttonsToLight := orAggregateAllLiveRequests()

	for i, row := range buttonsToLight {
		for j, val := range row {
			elevio.SetButtonLamp(elevio.ButtonType(j), i, val)
		}
	}
}

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

// Potential issue: If the elevator has been offline for too long, a direct state request might time out.
// Consider implementing a retry mechanism or fallback to a safe state.
//
// ResyncElevatorState should be triggered when an elevator is detected to be out of sync.
func ResyncElevatorState(elevatorID int) {
	// TODO:
	// 1. Broadcast a state request for the specified elevator.
	// 2. Wait for responses from peers or the master node.
	// 3. If a valid state is received, update the local state accordingly.
	// 4. If no response is received within a timeout, mark the elevator as offline and trigger order reassignments.
}

// Helps with improving error checking for receiving and processing state messages over UDP.
// (Not sure if this is needed but can be nice for testing at least, so we dont try to fix something that not broken)
func HandleStateReception() {
	// TODO:
	// 1. In the ReceiveStates loop, check for errors on conn.Read.
	// 2. If an error occurs, log it and possibly break the loop or retry.
	// 3. Validate the length of the received data before attempting deserialization.
}

// TODO maybe refator into separate file
func (e *elevatorState) GetID() int {
	return int(e.id)
}

func (e *elevatorState) GetFloor() int {
	return int(e.currFloor)
}

func (e *elevatorState) GetDirection() elevio.MotorDirection {
	return e.currDirection
}

func (e *elevatorState) GetRequests() [][3]bool {
	requestsCopy := make([][3]bool, len(e.request))
	for i, requests := range e.request {
		requestsCopy[i][elevio.BT_HallUp] = requests[elevio.BT_HallUp]
		requestsCopy[i][elevio.BT_HallDown] = requests[elevio.BT_HallDown]
		requestsCopy[i][elevio.BT_Cab] = requests[elevio.BT_Cab]
	}
	return requestsCopy
}
