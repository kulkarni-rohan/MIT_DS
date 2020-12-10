# Paper: Object Storage on CRAQ
## Basic System Model
### I. Interface and Consistency Model
#### A. Interface
1. write(objID, V): The write (update) operation stores the value V associated with object identifier ob jID.
2. V read(objID): The read (query) operation retrieves the value V associated with object id ob jID.
#### B. Consistency Model
1. Strong Consitency: all read and write operations to an object are executed in some sequential order
2. Eventual Consistency: Eventually-consistent reads to different nodes can return stale data for some period of inconsistency
### II. Chain Replication
1. write: process by head, propagates to tail
2. read: from tail
### III. Chain Replication with Apportioned Queries
#### A. steps
1. node in CRAQ can store multiple versions of an object, each including a monotonically-increasing version number and an additional attribute whether the version is `clean` or `dirty`. All versions are initially marked as clean
2. node receive new version, append to latest version: not tail -> mark dirty, propagates the write to successor; is tail -> mark as clean, notify other nodes to commit
3. when an acknowledgement message for an object version arrives at a node, the node marks the object version as clean. The node can then delete all prior versions of the object
4. receive read: if lastest version is clean -> return; dirty -> request the tail for the lastest committed version number and submit the corresponding copy.
5. note: read operations are serialized with respect to the tail
#### B. Improvements
1. Read-Mostly Workloads
2. Write-Heavy Workloads: lighter-weight than full reads
### IV.Consistency Models on CRAQ
1. Strong Consistency
2. Eventual Consistency
3. Eventual Consistency with Maximum-Bounded Inconsistency: allows read to return newly written objects before they commit, but only to a certain point. Limit can be based on time or version numbers
### V. Failure Recovery in CRAQ
1. head fail -> successor takes over
2. tail fail -> predecessor takes over
3. node joining or failing from within the middle of the chain must insert themselves between two nodes, much like a doubly-link list
## Scaling CRAQ
### I. Chain Placement Strategies
#### A. common situations
1. Most or all writes to an object might originate in a single datacenter
2. Some objects may be only relevant to a subset of datacenters
3. Popular objects might need to be heavily replicated while unpopular ones can be scarce
#### B. two-level naming hierarchy for objects
1. chain identifier
2. key identifier
#### C. ways of specifying application requirements
1. Implicit Datacenters & Global Chain Size: {num_datacenters, chain_size}
2. Explicit Datacenters & Global Chain Size: {chain_size, dc1, dc2, ..., dcN}
3. Explicit Datacenter Chain Size: {dc1, chain_size1, ..., dcN, chain_sizeN}
#### D. master datacenter
1. during transient failures, writes to the chain will only be accepted by master dc.
2. when master if not defined, writes will only continue in a partition if the partition contains a majority of the nodes in the global chain
### II. CRAQ within a Datacenter
1. place chains within a datacenter using consistent hashing, mapping potentially many chain identifiers to a single head node
2. improves the potential for parallel system recovery
3. it comes at the cost of increased centralization and state
### III. CRAQ Across Multiple Datacenters
1. applications can minimize write latency by carefully selecting the order of datacenters that compromise a chain
2. we can ensure that a single chain crosses the network boundary of a datacenter only once in each direction
3. latency of write will increase as more datacenters are added to the chain, but writes can be pipelined
### IV. ZooKeeper Coordination Service
1. CRAQ nodes are guaranteed to receive notification when nodes are added to or removed from a group
2. node can be notified when metadata in which it has expressed interest changes
3. ZooKeeper nodes use an atomic broadcast similar to two-phase-commit.
4. ZK is not optimized for running in a multi-datacenter environment
5. CRAQ nodes always receive notifications from local ZK nodes, and they are further notified only about chains and node lists that are relevant to them
6. to remove the redundancy of cross-datacenter ZK traffic one could build a hierarchy of Zookeeper instances: Each datacenter could contain its own local ZooKeeper instance (of multiple nodes), as well as having a representative that participates in the global ZooKeeper instance (perhaps selected through leader election among the local instance).
## Extensions
### I. Mini-Transactions on CRAQ
whole-object read/write of an object store may be limiting
#### A. Single-Key Operations
1. Prepend/Append: head simply apply operation to the latest version even if it's dirty, then propagate to a full replacement
2. Increment/Decrement: same as 1
3. Test-and-Set: head checks if its most recent committed version number equals to the number in the operation. If no outstanding uncommitted versions of the object, the head accepts the operation and propagates an update down the chain. If there are outstanding writes, we simply reject the test-and-set operation, and clients are careful to back off their request rate if continuously rejected. Alternatively, the head could "lock" the object by disallowing writes until the object is clean
#### B. Single-Chain Operations
##### a. Mini-Transaction
1. mini-transaction is defined by a compare, read, and write set.
2. a compare set tests the values of the specified address location, and if they match the provided values, executes the read and write operations. Typically designed for settings with low write contention
3. Sinfonia's mini-transactions use an optimistic two-phase commit protocol. The prepare message attempts to grab a lock on each specified memory address. If all addresses can be locked, the protocol commits; otherwise, the participant releases all locks and retries later
##### b. CRAQ chain topology
1. application can designate multiple objects be stored on the same chain in such a way that preserves locality
2. objects sharing the same chainid will be assigned the same node as their chain head, reducing the two-phase commit to a single interaction because only one head node is involved.
3. head controls write access to all of a chain's keys, as opposed to all chain nodes
4. trade-off: head needs to wait for keys in the transaction to become clean
#### C. Multi-Chain Operations
1. optimistic two-phase protocol need only be implemented with the chain heads, not all involved nodes.
2. chain heads can lock any keys involved in the mini-transaction until it is fully committed
3. application writes should be careful with the use of extensive locking and mini-transactions: they reduce the write throughput of CRAQ as writes to the same object can no longer be pipelined, one of the very benefits of chain replication
### II. Lowering Write Latency with Multicast
1. within a datacenter -> network-layer multicast protocol
2. head multicast writes to entire chain, tail multicast acknowledgement message to entire chain
