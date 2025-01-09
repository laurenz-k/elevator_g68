# Results

## Shared Variable Task

### No synchronization
We would expect the result to always be 0. However, the `i++` and `i--` operations are not atomic. It consists of three operations: 
1. Read value of `i`
2. increment/decrement `i` 
3. write the new value back to `i`

Since there is no locking a thread might increment/decrement and outdated value of `i` or decrement an outdated value of `i` and write the outdated value back to `i`. Therefore some increments/decrements will be lost. We only know for sure i will be in the range `[-1,000,000 to 1,000,000]`.

## Synchronization

### Why did I choose a mutex over semaphore in C?
Semaphores usually store a count of resources that are available. Since in our case we only have one resource (`i`), a mutex-lock is more appropriate.