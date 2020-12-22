# Paper: No Compromises: distributed transactions with consistency, availability, and performance
## Hardware Trends
### I. Non-volatile DRAM
### II. RDMA Networking
## Programming Model and Architecture
1. FaRM provides applications with the abstraction of global address space that spans machines in a cluster
2. An application thread can start a transaction at any time and it becomes the transaction's coordinator.
3. FaRM transactions use optimistic concurrency control: updates are buffered locally during execution and only made visible to other transactions on a successful commit.
4. strict seriazability of all successfully committed transactions.
5. FaRM guarantees that individual object reads are atomic, that they read only committed data
6. does not guarantee atomicity across reads of different objects but, in this case, it guarantees that the transaction does not commit ensuring committed transactions are strictly serializable.
7. this allow us to defer consistency checks until commit time instead of re-checking consistency on each object read.
8. FaRM API also provides lock-free reads, which are optimized single-object read only transactions, and locality hints, which enable programmers to co-locate related objects on the same set of machines.
9. use ZooKeeper to ensure machines agree on the current configuration and to store it, as in Vertical Paxos.
10. global address space in FaRM consists of 2GB regions, each replicated on one primary and f backups
11. reads from the primary copy
12. each object has a 64-bit version that is used for concurrency control and replication
13. mapping of a region identifier to its primary and backups is maintained by the CM and replicated with the region.
14. assign mapping -> 2PC
## Distributed Transactions and Replication
1. Lock: coordinator writes a LOCK record to the log on each machine that is a primary for any written object. This contains the versions and new values of all written objects on that primary. Try to lock using compare-and-swap, and send back a message reporting whether all locks were successfully taken
2. Validate: coordinator performs read validation by reading, from their primaries, the versions of all objects that were read but not written by the transaction. If any object has changed, validation fails and the transaction is aborted
3. Commit Backups: send COMMIT-BACKUP and wait for ACK
4. Commit Primary: after all COMMIT-BACKUP writes have been ACKed. Coordinator writes a COMMIT-PRIMARY record to the logs at each primary. Primaries process these records by updating the objects in place, incrementing their versions, and unlocking them, which exposes the writes committed by the transaction
5. Truncate: Backups and Primaries keep the records in their logs until they are truncated. Coordinator truncates logs lazily, it piggybacks identifiers of truncated transactions in other log records
#### Performance
1. Paxos: 2f + 1 replicas tolerates f failures, send 4P (2f + 1) messages
2. FaRM: f + 1 copies, Pw (f + 3) one-sided RDMA writes; Pr one-sided RDMA reads
## Failure Recovery
### I. Failure Detection
1. 5ms lease
2. dedicated lease manager thread running at the highest user-space priority (31 on Windows), uses interrupts instead of polling
### II. Reconfiguration
1. Suspect: CM -> lease expires -> block all external client requests; non-CM -> lease expires -> ask one of a small number of "backup CMs" to initiate reconfiguration -> if configuration unchanged after a timeout period -> reconfigure itself
2. Probe: new CM issues an RDMA read to all the machines in the configuration except the machine that is suspected
3. Update Configuration: update config data in ZK, <c + 1, S, F, CMid>
4. Remap Regions: restore the number of replicas to f + 1
5. Send New Configuration: NEW-CONFIG to all the machines in the configuratino with the configuration identifier
6. Apply New Configuration: update its current configuration identifier and its cached copy of the region mappings, and allocates space to hold any new region replicas assigned to it. From this point, it does not issue new requests to machines that are not in the configuration and it rejects read responses and write ACKs from those machines. It also starts blocking requests from external alients. Machines reply to the CM with a NEW-CONFIG-ACK message. If the CM has changed, this both grants a lease to the CM and requests a lease
7. Commit New Configuration: Once CM receives NEW-CONFIG-ACK messages from all machines in the configuration, it waits to ensure that any leases granted in previous configurations to machines no longer in the configguration have expired. The CM then sends a NEW-CONFIG-COMMIT to all the configuraiton members that also acts as a lease grant.
### III. Transaction State Recovery
1. Block access to recovering regions: primary fail -> one backup takeover -> block until all transactions that updated it have been reflected at the new primary
2. Drain logs: reject messages form old configuration. draining logs to ensure all relevant records are processed during recovery
3. Find recovering transactions: during log draining, the transaction identifier and list of updated region identifiers in each log record in each log is examined to determine the set of recovering transactions. All machines must agree on whether a given transaction is recovering transaction or not. We achieve this by piggybacking some extra metadata on the communication during the reconfiguration phase. CM reads the LastDrained variable at each machine as part of the probe read. A transaction that started committing in configuration c - 1 is recovering in configuration c unless: for all regions r containing objects modified by the transaction LastReplicaChange[r] < c
4. Lock recovery: primary waits until the local machine logs have been drained and NEED-RECOVERY messages have been received from each backup
5. Replicate log records: send backups REPLICATE-TX-STATE message for any transactinos that they are missing.
6. Vote: votes are sent by primaries of each region. Vote is `commit-primary` if any replica saw COMMIT-PRIMARY or COMMIT-RECOVERY. Otherwise, it votes `commit-backup` if any replica saw COMMIT-BACKUP and did not see ABORT-RECOVERY. Otherwise, it votes `lock` if any replica saw a LOCK record and no ABORT-RECOVERY. Otherwise, it votes `abort`
7. Decide: if it receives a `commit-primary` vote from any region, it commits. Otherwise, it waits all regions to vote and commits if at least one region voted `commit-backup` and all other regions voted `lock`, `commit-backup`, or `truncated. Otherwise it decides to abort
### IV. Recovering Data
1. FaRM must recover (re-replicate) data at new backups for a region to ensure that it can tolerate f replica failures in the future. Data recovery is not nessary to resume normal case operation, so we delay it until all regions become active to minimize impact on latency-critical lock recovery.
2. Each machine sends a REGIONS-ACTIVE message to the CM when all regions for which it is primary become active. After receiving all REGIONS-ACTIVE messages, the CM sends a message ALL-REGIONS-ACTIVE to all machines in the configuration.
3. A new backup for a region initially has a freshly allocated and zeroed local region replica. It divides the region across worker threads that recover it in parallel. 
4. Each recovered object must be examined before being copied to the backup. If the object has a version greater than the local version, the backup locks the local version with a compare-and-swap, updates the object state, and unlocks it.
### V. Recovering Allocator State
1. FaRM allocator splits regions into blocks (1MB) that are used as slabs for allocating small objects. It keeps two pieces of meta-data: block headers and slab free lists
2. Block headers are used in data recovery, the new primary sends them to all backups immediately after receiving NEW-CONFIG-COMMIT
3. slab free lists are kept only at the primary to reduce the overheads of object allocation.