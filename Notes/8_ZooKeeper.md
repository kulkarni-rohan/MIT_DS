# Paper: ZooKeeper: Wait-free coordination for Internet-scale
## Terminology
1. client: a user of the ZooKeeper service
2. server: a process providing the ZooKeeper service
3. znode: in-memory data node in the ZooKeeper data
4. data tree: a hierarchical namespace, which organizes znodes
5. session: clients establish a session when they connect to ZooKeeper and obtain a session handle through which they issue requests
## The ZooKeeper Service
### I. two types of znode:
1. Regular: create, delete explicitly
2. Ephemeral: either delete explicitly or remove automatically when session creates the node terminates
### II. flags
1. sequential flag: monotocically increasing counter appended to its name
2. watch flag: notify the client when the information returned has changed; one-time triggers associated with session
### III. Data model
1. a file system with a simplified API and only full data reads and writes, or a key/value table with hierarchical keys.
2. znodes map to abstractions of the client application: each client process pi creates a znode p_i under /app1, persists as long as the process is running
3. ZooKeepers allow clients to store some information that can be used for meta-data or configuration in a distributed computation. Metadata is associated with timestamps and version counters, which allow clients to track changes to znodes and execute conditional updates based on the version of the node
### IV. Sessions
has timeout
## Client API
### I. API
1. create(path, data, flags)
2. delete(path, version): deletes if version matchs
3. exists(path, watch): watch is bool
4. getData(path, watch)
5. setData(path, data, version)
6. getChildren(path, watch)
7. sync(path): waits for all pending writes to complete
### II. explain
1. all methods have both a sync and async version
2. sync: not concurren, makes the necessary ZooKeeper call and blocks
3. async enables an application to have both multiple outstanding ZooKeeper operations and other tasks executed in parallel
4. ZooKeeper does not use handles to access znodes. Each request includes the full path of the znode being operated on.
## ZooKeeper Guarantees
1. Linearizable writes: all requests that update the state of ZooKeeper are serializable and respect precedence.
2. FIFO client order: all requests from a given client are executed in the order they were sent by the client
3. if a majority of ZooKeeper servers are active and communicating the service will be available
4. if the ZooKeeper service responds successfully to a change request, that change persists across any number of failures as long as a quorum of servers is eventually able to recover
## Examples of Primitives
### I. Configuration Management
1. dynamic configuration
2. set watch flag to true, read config file, upon notified and read new configuration, again set the watch flag to true
### II. Rendezvous
1. created by the client
2. passes the full pathname of zr as a startup parameter of the master and worker processes.
3. when master starts, it fills in zr, with information about addresses and ports it is using.
4. when worker starts, they read zr with watch set to true
5. if zr has not been filled in yet, the worker waits to be notified when zr is updated
### III. Group Membership
1. zg present a group
2. when a process member under the group starts, it creates an ephemeral child znode under zg.
3. they can name the znodes either using their names (must be unique) or set sequential flag
4. after created, it does not to do anything, just wait for the process to fail or ends, it will be auto removed
5. can obtain group information y simply listing the children of zg.
6. if want to monitor changes, set the watch flag to true when read zg
### IV. Simple Locks
1. lock file: represented by a znode.
2. create lock: create znode with ephemeral flag, if not success, read with watch flag to wait anothe to release the lock
### V. Simple Locks without Herd Effect
```
Lock
1 n = create(l + “/lock-”, EPHEMERAL|SEQUENTIAL)
2 C = getChildren(l, false)
3 if n is lowest znode in C, exit
4 p = znode in C ordered just before n
5 if exists(p, true) wait for watch event
6 goto 2

Unlock
1 delete(n)
```
### VI. Read/Write Locks
```
Write Lock
1 n = create(l + “/write-”, EPHEMERAL|SEQUENTIAL)
2 C = getChildren(l, false)
3 if n is lowest znode in C, exit
4 p = znode in C ordered just before n
5 if exists(p, true) wait for event
6 goto 2
Read Lock
1 n = create(l + “/read-”, EPHEMERAL|SEQUENTIAL)
2 C = getChildren(l, false)
3 if no write znodes lower than n in C, exit
4 p = write znode in C ordered just before n
5 if exists(p, true) wait for event
6 goto 3
```
### VII. Double Barrier
1. used to sync the beginning and the end of a computation
2. process can enter the barrier when the number of child znodes of b exceeds the barrier threshold.
3. process can leave when all of the processes have removed their children
4. use watches to efficiently wait for enter and exit.
## ZooKeeper Implementation
1. if write request: use an agreement protocol, finally servers commit changes to the ZooKeeper database fully replicated across all servers of the ensemble.
2. read requests, reads the state of the local database and generates a response to the request
3. replicated databse is an in-memory database containing the entire data tree, each node stores 1MB by default.
4. write requests: forwarded to leader, the followers receive message proposals consisting of state changes from the leader and agree upon state changes
### I. Request Processor
1. when leader receives write request, it calculates what the state of the system will be 
2. then transform it into a transaction that captures this new state
### II. Atomic Broadcast
1. Zab is an atomic broadcast protocol, uses simple majority quorums to decide on a proposal
2. leader executes the requests and broadcasts the change to the ZooKeeper state through Zab
3. Zab guarantees the changes broadcast by a leader are delivered in order they were sent and all changes from previous leaders are delivered to an established leader before it broadcasts its own changes
4. TCP for transport so message order is maintained by the network
5. use log to keep track of proposals as the write-ahead log for the in-memory database
6. may redeliver a message during recovery
### III. Replicated Database
1. periodic snapshots
2. redelivery of messages since the start of the snapshot, called fuzzy snapshot
3. do a DFS of the tree atoically reading each znode's data and meta-data and writing them to disk.
4. result may not correspond to the state of ZooKeeper at any point in time
### IV. Client-Server Interactions
1. read is handled locally in memory. Each read request is processed and tagged with a zxid that corresponds to the last transaction seen by the server
2. this zxid defines the partial order of the read requests with respect to the write requests
3. drawback: not guaranteeing precedence order for read operations, read may return a stale vlue. Should use sync.
4. sync: place sync operation at the end of the queue of requests between the leader and the server executing the call to sync.
5. if pending queue is empty, the leader needs to issue a null transaction to commit and orders the sync after that transaction. 
6. heartbeat: send heartbeat after the session has been idle for s/3 ms and switch to a new server if it has not heard from a server for 2s/3 ms. s is session timeout in ms.
