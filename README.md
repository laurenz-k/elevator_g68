# Elevator Controller - Group 68 

Our current approach and modules.

## Controller
Core elevator control loop that manages the movement and operation of a single elevator. Handles door control, responding to cab calls and external hall requests.

## Assigner
### `AssignRequest(ButtonEvent)`
When an elevator receives a hall call, we calculate the cost of each elevator to take this order. The elevator with the lowest cost gets announced via UDP message `<elevatorId, floor, buttonType>`.
If a transmission was not successful we retry until delivery is possible.

### `ReceiveAssignments(channel)`
Listens on UDP for assignment messages which are intended for this elevator. The elevator then updates it's requests matrix and broadcasts it out to other elevators. To avoid message drop risk we broadcast multiple times. For now 10 times seems sufficient since even at 50% package loss we still achieve 1 - 0.5^10 = 99.9% delivery guarantee.
After this it's safe to light the elevator's button since other elevators are aware of the orders existance and can take over.

## StateSync
### `broadcastState(elevatorPtr)`
Broadcasts current elevator's state to all other elevators `<nonce/ts, elevator_id, floor, direction, requests>` via UDP every 100ms.

### `receiveStates()`
Listens for incoming state updates and updates the state of other elevators.

### `monitorFailedSyncs()`
Runs periodically (every 1 second) to detect elevators that haven't shared their state in the last second. If this is the case we reassign all of the failed elevators requests. 
Since in theory multiple elevators could recognize the failure at the same time which might lead to a scenario where an order is assigned to multiple other elevators. While this is not ideal it does not violate the service guarantee since at least one elevator will take over the order. 
We are considering adapting the reassignment scheme such that first an elevator waits a random time and then checks if the order has already been reassigned by another elevator. If it has not been reassigned yet we can assume that this elevator is the first to reassign. 
