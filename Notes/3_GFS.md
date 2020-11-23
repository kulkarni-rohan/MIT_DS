# Paper: The Google File System
## Design Overview
### I. Assumptions
1. often fail
2. larger files are common (multi-GB)
3. two kinds of read: large streaming reads and small random reads
4. many large, sequential writes that append data to files. Once written, seldom modified
5. files are often used as producer-consumer queue. Concurrent append efficiency is essential
6. high sustained bandwidth is more important than low latency
### II. Interface
`create` `delete` `open` `close` `read` `write` `snapshot` `record append`
### III. Architecture
1. `master`, multiple `chunkservers`, accessed by multiplt `clients`
2. files are divided into fixed-size chunks, identified by 64bit `chunk handle`.
3. master maintains all file system metadata and controls system wide activities, master periodically communicates with each chunkserver in `HeartBeat` messages to give it instructions and collect its state.
4. clients ask master for metadata, then communicates directly with chunkserver
5. neither clients nor chunkserver caches file data. Clients cache metadata though.
### IV. Single Master
1. using the fixed chunk size, client computes the chunk index.
2. client sends request (file name, chunk index) to master
3. master replies (chunk handle, chunk locations)
4. client caches this info, using file name and chunk index as the key
5. client sends request (chunk handle, byte range) to one of the replicas (usually closest one).
6. further reads of the same chunk don't involve master, until cached info expires or file is reopened.
7. in fact, client typically asks for multiple chunks, it's batched to reduce overhead
### V. Chunk Size
one of the key design parameters, 64MB here
#### A. large chunk advantages
1. reduces clients' need to interact with the master
2. many operations are more likely to target at the same chunk, can reduce network overhead by keeping a persistent TCP connection to the chunkserver over an extened period of time
3. reduces metadata size, easier for master to store in memory
#### B. disadvantage
small file consists of small number of chunks, the chunkserver becomes hot spot
### VI. Metadata
1. three types: a. the file and chunk namespaces. b. mapping from files to chunks. c. locations of each chunk's replicas.
2. all is kept in master's memory
3. first two also kepts persistent by logging mutations to an `operation log` stored on the master's local disk and replicated on remote machines
#### A. In-Memory Data Structures
1. fast for master operations, including gc, re-replication, chunk migration
2. 64bytes metadata for each 64MB, capacity is not an issue
#### B. Chunk Locations
1. master doesn't keep a persistent record, instead it simply polls chunkservers for that info at startup. Then `HeartBeat` messages was used to update it.
2. this approach eliminates the problem of syncing master and chunserver
3. chunkserver has the final word over what chunks it does or does not have in its on disks
#### C. Operation Log
1. replicate it on multiple remote machines
2. respond to a client operation only after flushing the corresponding log record to disk both locally and remotely. 
3. master batches several log records together before flushing
4. recovers file system by replaying the log.
5. keep it small to reduce startup time
6. master checkpoints its state whenever the log grows beyond a certain size
7. checkpoint is in a compact B-tree like form, can directly mapped into memory and used for namespace lookup without extra parsing, this speeds up recovery and improves availability
8. master switched to a new log file and creates the new checkpoint in a separate thread, this prevents delaying incoming mutations
9. only keeps latest complete checkpoint and subsequent log files
### VII. Consistency Model
GFS has a relaxed consistency model
#### A. Guarantees by GFS
1. consistent: all clients see the same data
2. defined: consistent and client will see what the mutations writes in its entirety
3. data mutations includes `write` and `record append`. 
4. `write` cause data to be written at "app-specified" offset (which make region consistent but not defined in concurrency)
5. `record append` make sure data to be appended `at least once` (result of retry upon unsuccessful append), offset is chosen by GFS. This is because GFS may insert padding or record dupplicates in between, which occupy region but inconsistent and usually dwarfed by the amount of user data
6. after successful mutations, the mutated region is defined. GFS achieved this by a. same mutation order b. mutation version
7. stale replicas are garbage. client clears this after a. timeout b. next open of the file
8. GFS identifies failed chunkservers by regular handshakes and detects data corruption by checksumming
#### B. Implications for Applications
GFS accommodates the relaxed consistency model with a few techniques
1. relying on appends rather than overwrites
2. checkpointing
3. writing self-validating, self-identifying records
## System Interactions
### I. Leases and Mutation Order
1. one of the replicas is called `primary`, master grants lease to it
2. lease defines mutation order
3. initial timeout is 60s, can extend, sent together with heartbeat. 
#### Steps
1. client ask master for lease location
2. master replis with primary and secondaries locations
3. client push data to all the replicas (in any order)
4. after all the replicas acknowledged, client sends a write request to the primary
5. primary forwards the write request to all secondary replicas
6. secondaries replies upon completion
7. if unsuccessful, return to client, client retry
### II. Data Flow
1. data is pushed linearly
2. reduce latency by pipelining the data transfer over TCP connections.
### III. Atomic Record Appends
1. if data too big -> pads it to maximum of current chunk then append, then return and ask client to retry in next chunk.
2. GFS does not guarantee that all replicas are bytewise identical, it only guarantees that the data is written at least once as an atomic unit.
3. record append is restricted to one fourth of the maximum chunk size to keep worst case fragmentation at an acceptable level
### IV. Snapshot
1. copy-on-write
2. copy to the same replica. after copy action, master doesn't really copy, it increment the ref count of the chosen chunks and revokes the lease of corresponding chunks. Client can read, but if client want to write, it must ask for lease, at this time, master do the real copy of the chunk and decrement the ref count
## Master Operation
### I. Namespace Management and Locking
