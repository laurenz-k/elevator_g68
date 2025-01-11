Exercise 1 - Theory questions
-----------------------------

### Concepts

#### What is the difference between *concurrency* and *parallelism*?
- Concurrency is multiple tasks making progress at the same time, but not necessarily simultaneously.
- Parallelism is when multiple tasks are executed simultaneously.

#### What is the difference between a *race condition* and a *data race*?
- Data Race: When two or more threads access the same memory location concurrently, and at least one of the accesses is a write. This leads to non deterministic behavior, since depending on the order of execution, the result can be different. Our initial *shared variable* task exhibits a data race.
- Race Condition: Is a situation in which a software system's behavior is dependent on the sequence or timing of uncontrollable events. Examples could be the execution order of threads, or the order of arrival of messages.
 
#### *Very* roughly - what does a *scheduler* do, and how does it do it?
A scheduler decides which users (e.g. threads) run on a resource (e.g. CPU) and for how long. 
It does the following:
- It keeps track of all tasks
- Decides which tasks to run
- Runs the task for a certain amount of time
- Switch to the next task


### Engineering

#### Why would we use multiple threads? What kinds of problems do threads solve?

We need threads whenever we need concurrent progress for multiple parts of our program. Common examplels are servers which need to be responsive to incoming requests while handling them, UI which needs to stay alive while executing a long running task in the background.

Of course this could also be achieved using processes, however you should not create too many processes since they are heavier and slower to swap. Additionally, processes do not share memory space so communication is a bit trickier, e.g. syscall to allocate shared memory, pipes, sockets, ... 

On multicore systems threads might also improve performance since threads can be parallelized across cores.

#### Some languages support "fibers" (sometimes called "green threads") or "coroutines"? What are they, and why would we rather use them over threads?
Coroutines are very similar to threads. However, unlike OS-level threads, coroutines are managed entirely by the programming language runtime. Rather than switching by preemption/interrupt by the OS, a coroutine can explicity yield control to another (often via async/await).
This makes switches between coroutines more efficient.

However, this also means that multiple coroutines cannot escape their thread, and therefore performance gains are not possible on multi-core systems. 

Therefore coroutines are perfect for IO heavy tasks (lots of waiting block time), and less suitible for CPU intensive tasks.

**Does creating concurrent programs make the programmer's life easier? Harder? Maybe both?**
Depends on the application. Concurrent programming is essential for many modern applications - like building a server that can handle thousands of simultaneous user request. 
However it also introduces new challenges, specifically sharing resources, preventing deadlocks, ... 
Also testing becomes harder, since some bugs might only appear under certain timing conditions.

**What do you think is best - *shared variables* or *message passing*?**
I think message passing feels very intuitive. However in some case shared variables might be better/more performant. Like in our counter example it would be more performant for each thread to update the variable atomically, than aggregating the update operations in a server and executing updates from there.
