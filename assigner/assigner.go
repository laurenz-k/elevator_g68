package assigner

import (
	"elevator/elevio"
	"net"
	"time"
)


// TODO: Add a voting system for master election (RAFT)
// Add RAFT in a seperate file? It is not super easy to implement, but it would be nice for our system


var assignmentChan chan Assignment
		
const broadcastAddr = "255.255.255.255"
const broadcastPort = "20068"

type Assignment struct {
	ElevatorID int
	Floor int
	Button ButtonType
}


// Given a call (floor, button) and a set of elevator states, returns the best elevator ID.
func Cost(call elevio.ButtonEvent, aliveElevators []int) int {
    // TODO:
    // 1. For each elevator in aliveElevators, compute a cost based on:
    //    - Distance to the call floor
    //    - Current direction / load
    //    - Other calls in queue
	//	  An example cost function exists on blackboard, but would have to be rewritten in Go. 
    // 2. Pick the elevator with the lowest cost. Break ties by elevator ID if needed.
    // 3. Return the chosen elevator ID.
    return 0
}


func Assign(request elevio.ButtonEvent) {
	// TODO, Jakob, make cost function
	//Check if already assigned to an alive elevator - if so, do nothing
	//Obtain states of alive elevators, calculate their costs. Lowest cost wins. In a draw, lowest/highest ID wins.
	aliveIDs := GetAliveElevatorIDs()
	//Go through the costs of all elevators in loop with. Lowest wins.

	var winnerElevatorID int 

	addr := broadcastAddr + ":" + broadcastPort
	conn, _ := net.Dial("udp", addr)
	
	defer conn.Close()
	//

	assignment := Assigment{
		ElevatorID: winnerElevatorID,
		Floor: request.Floor,
		Button: request.Button,
	}

	conn.Write(serializeAssignment(assignment))
}

func serializeAssignment(assignment Assignment) []byte {
	buf := make([]byte, 128)
	buf = append(buf, byte(assignment.ElevatorID))
	buf = append(buf, byte(assignment.Floor))
	buf = append(buf, byte(assignment.Button))
	return buf
}

func deserializeAssignment(m byte[]) Assignment {  // @JakobSO Is this right? a list of bytes is defined []byte in Golang afaik
	assignment := Assignment{
		ElevatorID: m[0],
		Floor: m[1],
		Button: m[3],
	}
	return assignment
}

func ReceiveAssignments(thisElevatorID int) {
	addr, _ := net.ResolveUDPAddr("udp", broadcastAddr + ":" + broadcastPort)
	conn, _ := net.ListenUDP("udp", addr)
	
	defer conn.Close()
	
	buf := make([]byte, 128)
	
	for {
		n, _, _ := conn.ReadFromUDP(buf)
		assignment := deserialize(buf[:n])
		if assignment.ElevatorID == thisElevatorID {
			assignmentChan <- assignment
		}
	}
	
}

// This function should be called by non-master elevators whenever a new hall call is registered.
// The master should then evaluate the call and assign it to an elevator (via Assign).
func NotifyMasterOfNewHallCall(elevID int, floor int, button elevio.ButtonType) {
    // TODO: 
    // 1. Send a message (UDP, TCP, or statesync) to the master elevator indicating a new hall call.
    // 2. The master, upon receiving this, will run the assignment algorithm (cost function, etc.).
    // 3. If this elevator is itself the master, it might short-circuit and just call `Assign(...)`.
}

// If the disconnected elevator recovers at the same time you're reassigning tasks
// you might end up with duplicate assignments. Not too bad of a problem, since were not losing any calls.
//
// This function should be called when an elevator times out or fails a heartbeat.
func ReassignTasksForDisconnectedElevator(disconnectedID int) {
    // TODO:
    // 1. Retrieve all tasks currently assigned to 'disconnectedID'.
    // 2. Mark them as unassigned or move them into a queue of unassigned tasks.
    // 3. Re-run the assignment logic (cost function) for each of those tasks.
}

// The elevator is disconnected from the network but can still move locally.
// Its rare but possible. If we let it take hall calls we cannot ensure it will not lose them. 
// We might want to let it handle cab calls only. 
// (iirc Task description said elevators should be able to run "solo mode" if they get isolated)
//
// This function is called periodically if the elevator is offline, to decide whether it should
// handle new calls locally or remain idle.
func HandleLocalCallsWhenIsolated(elevID int) {
    // TODO:
    // 1. Check if we have connectivity to the master or any peer.
    // 2. If fully isolated, handle cab calls in a minimal way (stop at floors pressed inside the cabin).
    // 3. Log or queue hall calls so they can be broadcast if/when we reconnect? 
	// (Can cause duplicates if buttons get pressed on other elevators)
}

// If the master is down or busy, or if theres any scenario where we cant immediately assign
// a hall call, we can store it in a FIFO queue. Once the master is ready or a new master
// is elected, we pop from the queue and run the assignment. This way we dont lose any button presses
// (not super important as long as we dont light up the button before losing it!)
//
// Insert a new call into the unassigned queue, to be handled when a master is present.
func AddUnassignedTask(call elevio.ButtonEvent) {
    // TODO:
    // 1. Push the call onto a queue or list of unassigned tasks.
    // 2. Optionally broadcast that a new unassigned task is pending (so the master can pick it up).
}
