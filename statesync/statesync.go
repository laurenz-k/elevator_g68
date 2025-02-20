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
const broadcastPort = "15001"
const interval = 100 * time.Millisecond
const syncTimeout = 1 * time.Second

var mtx sync.RWMutex

// We store states by elevator id (index in the slice).
var states = make([]*elevatorState, 0, 10)

// ButtonPressChan is used to notify the controller to reassign orders
// when an elevator is detected as failed.
var ButtonPressChan chan elevio.ButtonEvent

type elevatorState struct {
	id            uint8
	nonce         uint32
	currFloor     uint8
	currDirection elevio.MotorDirection
	request       [][2]bool
	lastSync      time.Time
}

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
	}
}

// MonitorFailedSyncs iterates through the stored states every second.
// If an elevator has not sent an update within syncTimeout,
// its orders are reassigned via ButtonPressChan.
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

// GetState returns the stored state for the elevator with the given id.
// Returns nil if no such state exists.
func GetState(elevatorID int) *elevatorState {
	mtx.RLock()
	defer mtx.RUnlock()
	if elevatorID < len(states) {
		return states[elevatorID]
	}
	return nil
}

// GetAliveElevatorIDs returns a slice of elevator ids for which
// a state update has been received within syncTimeout.
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

func deserialize(m []byte) *elevatorState {
	elevatorState := &elevatorState{
		id:            m[0],
		nonce:         binary.LittleEndian.Uint32(m[1:5]),
		currFloor:     m[5],
		currDirection: elevio.MotorDirection(int8(m[6])),
		request:       make([][2]bool, 0, 128),
	}

	offset := 7
	for i := offset; i < len(m); i += 2 {
		currRow := [2]bool{m[i] == 1, m[i+1] == 1}
		elevatorState.request = append(elevatorState.request, currRow)
	}

	return elevatorState
}

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
