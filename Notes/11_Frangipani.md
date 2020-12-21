# Paper: Frangipani: A Scalable Distributed File System
## System Structure
### I. Component
1. user program access Frangipani through the standard operating system call interface
2. changes to file contents are staged through the local kernel buffer pool and are not guaranteed to reach nonvolatile storage until the next applicable `fsync` or `sync`
3. all the file servers read and write the same file system data structures on the shared Petal disk, but each server keeps its own redo log of pending changes in a distinct section of the Petal disk
4. logs are kept in Petal
5. Frangipani servers only communicate with Petal and the lock service
### II. Security and the Client/Server Configuration
only run on trusted operation systems
### III. Dicussion
#### A. Strength
1. allows transparent server addition, deletion, and failure recovery
2. create consistent backups
#### B. Problems
1. logging sometimes occurs twice, once to the Fragipani log and once again within Petal itself
2. Frangipani does not use disk location information in placing data because Petal virtualizes the disks
3. Frangipani locks entire files and directories rather than individual blocks
## Disk Layout
See Figure 4
## Logging and Recovery
1. use write-ahead redo logging of metadata; userdata is not logged
2. Gragipaniserver has its own private log in Petal, periodically written in the same order that the updates they describe were requested
3. logs are bounded in size - 128KB
4. after log processing is finished, the recovery demon releases all its locks and frees the log (doesn't release when crash)
5. monotonically increasing log sequence number to each 512B block of the log to ensuer that recovery can find the end of the log
6. how Fragipani's locking protocol ensures the logging and recovery work correctly in the presence of multiple logs: a. updates requested to the same data by different servers are serialized b. recovery applies only updates that were logged since the server acquired the locks that cover them, and for which it still holds the locks: recovery never replays a log record describing an update that has already been completed - use version number. c. at any time only one recovery demon is trying to replay the log region of a specific server
## Synchronization and Cache Coherence
1. write lock writes dirty data to disk before releasing
2. divides the on-disk structures into logical segments with locks for each segment
3. each log is a single lockable segment, because logs are private; bitmap also divided into segments that are locked exclusively; each file, directory or symlink is one segment -> one lock protects both the inode and any file data it points to. This per-file lock granularity is appropriate for engineering workloads where files rarely undergo concurrent write-sharing. Other workloads may require finer granularity locking
4. avoid deadlock by globally ordering these locks and acquiring them in two phases: a. a server determines what locks it needs. b. sorts the locks by inode address and acquire each lock in turn
## The Lock Service
### I. Lease
1. when a client first contacts the lock service, it obtains a lease
2. each lease has an expiration time, client must renew its lease before the expiration time, or the service will consider it to have failed
3. fail to renew -> server discardsall its locks and the data in its cache; if cache is dirty, Frangipani turns on an internal flag that causes all subsequent requests from user programs to return an error. The file system must be unmounted to clear this error condition
### II. Lock Service
1. lock service implementation is fully distributed for fault tolerance and scalable performance. It consists of a set of mutually cooperating lock servers, and a clerk module linked into each Frangipani server. 
2. Lock service organizes locks into tables. Each file system has a table associated with it. 
3. When Frangipani FS is mounted, the Frangipani server calls into the clerk, which opens the lock table, gives the clerk a lease identifier on a successful open. 
4. When file system unmounted, the clerk closes the lock table
5. basic message types: request, grant, revoke and release.
6. discard lock if not used for a long time
7. Paxos to replicate global state information across servers
## Adding and Removing Servers
1. add: new server need only be told which Petal virtual disk to use and where to find the lock service
2. remove: remove directly
## Backup
snapshot: exact same copy of a virtual disk at any point in time, copy-on-write
