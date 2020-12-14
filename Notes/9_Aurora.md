# Paper: Amazon Aurora: Design Considerations for High Throughput Cloud-Native Relational Databases
## Abbreviations
AZ: Availability Zone, subset of a region that is connected to other AZs in the region through low latency links but is isolated for most faults.\
CPL: Consistency Point LSN\
EBS: Amazon Elastic Block Store\
EC2: Amazon Elastic Compute Cloud\
GC: Garbage Collection\
HM: Host Manager\
LAL: LSN Allocation Limit\
LSN: Log Sequence Number\
MTTF: Mean Time to Failure\
MTTR: Mean Time to Repair\
MTR: mini-transaction\
PG: Pretection Groups\
PGMRPL: Protection Group Min Read Point LSN\
RDS: Relational Database Services\
S3: Amazon Simple Storage Services\
SCL: Segment Complete LSN\
VCL: Volume Complete LSN: the highest LSN for which it can guarantee availability of all prior log records\
VDL: Volumn Durable LSN: highest CPL that is smaller than or equal to VCL\
VPC: Virtual Private Cloud\
WAL: write-ahead log
## Durability at Scale
### I. Replication and Correlated Failures
#### A. two rules
1. Vr + Vw > V
2. Vw > V/2
#### B. Aurora
1. 2/3 quorums are inadequate
2. use 4/6 instead, Vw = 4, Vr = 3, V = 6
3. replicate items 6 ways across 3 AZs with 2 copies of each item in each AZ
### II. Segmented Storage
1. what we need: MTTR << MTTF
2. partition database volumn into small fixed size segments, currently 10GB. 
3. Each replicated 6 ways into PG so that each PG consists of six 10GB segments, organized across three AZs
4. a storage volumn is a concatenated set of PGs, physically implemented using a large fleet of storage nodes that are provisioned as virtual hosts with attached SSDs using EC2
5. segments are now our unit of idependent background noise failure and repair.
### III. Operational Advantages of Resilience
1. Once one has designed a system that is naturally resilient to long failures, it is naturally also resilient to shorter ones
2. execute updates one AZ at a time
## The Log is the Database
### I. The Burden of Amplified Writes
1. need to write redo log, binary log to S3, the modified data pages, a second temporary write of the data page (double-write) to prevent torn pages, finally the metadata files.
2. not desired in traditional relational database like MySQL: a. sequential and synchronous -> slow and vulnerable to failures, b. many different types of writes often representing the same information in multiple ways
### II. Offloading Redo Processing to Storage
1. only writes that corss the network are redo log records.
2. No pages are ever written from the database tier, the log applicator is pushed to the storage tier.
3. materialize databse pages in the background to avoid regenerating them from scratch on demand, only for page with long chain of modifications
4. this approach also improves availability by minimizing crash recovery time and eliminates jitter caused by background processes such as checkpointing, background data page writing and backups
5. crash recovery is spread across all normal foreground processing, any read request for a data page may require some redo records to be applied if the page is not current
### III. Storage Service Design Points
1. core tenet: minimize the latency of the foreground write request
2. move the majority of storage processing to backgorund
3. trade CPU for disk: do background service (such as GC) only when disk is approaching capacity
4. in Aurora: background processing has negative correlation with foreground processing, unlike a traditional database
#### various activities on the storage nodes: figure 4
1. receive log record and add to an in-memory queue
2. persist record on disk and acknowledge
3. organize records and identify gaps in the log since some batches may be lost
4. gossip with peers to fill in gaps
5. coalesce log records into new data pages
6. periodically stage log and new pages to S3
7. periodically garbage collect old versions, and finally
8. periodically validata CRC codes on pages
## The Log Marches forward
### I. Solution sketch: Asynchronous Processing
1. each log has an mono-inc LSN
2. during recovery, truncate all log record with an LSN larger than VCL
3. can further truncate more points by tagging log records and identifying them as CPLs, trucate all logs with LSN greater than the VDL
#### A. the way database and storage interacts
1. Each database-level transaction is broken up into multiple mini-transactions (MTRs) that are ordered and must be performed atomically.
2. Each mini-transaction is composed of multiple contiguous log records (as many as needed).
3. The final log record in a mini-transaction is a CPL.
### II. Normal Operation
#### A. Writes
1. for cuncurrent transactions, database allocates a unique ordered LSN for each log record subject to a constraint that no LSN is allocated with a value that is greater than VDL + LAL
2. each segment of each PG only sees a subset of log records in the volume that affect the pages residing on that segment. Each log record contains a backlink that identifies the previous log record for that PG.
3. backlinks can be used to track the point of completeness of the log records that have reached each segment to establish a SCL that identifies the greatest LSN below which all log records of the PG have been received
#### B. Commits
1. commits are completed async
2. the handling thread sets the transaction aside by recording its "commit LSN" as part of a seperate list of transactions waiting on commit.
3. the equivalent to the WAL protocol is based on completing a commmit if and only if the latest VDL is greater than or equal to the transaction's commit LSN
4. as VDL advances, the database identifies qualifying transactions that are waiting to be committed and uses a dedicated thread to send commit acknowledgements to waiting clients
#### C. Reads
1. pages are served from cache, if not in cache, send IO request
2. if cache is full, find a victim page to evict.
3. Aurora does not write page on eviction, instead it ensures a page in the buffer cache must always be of the latest version
4. evict a page from the cache only if its "page LSN" is greater than or equal to the VDL. a. all changes in the page have been hardened in the log b. on a cache miss, it is sufficient to request a version of the page as of the current VDL to get its latest durable version
5. The database does not need to establish consensus using a read quorum under normal circumstances. When reading a page from disk, the database establishes a read-point, representing the VDL at the time the request was issued. 
6. A page that is returned by the storage node must be consistent with the expected semantics of a mini-transaction (MTR) in the database. 
7. issue a read request directly to a segment whose SCL is greater than the read-point
8. guarantees no read page request with a read-point that is lower than the PGMRPL
#### D. Replicas
1. to minimize lag, the log stream generated by the writer and sent to the storage nodes is also sent to all read replicas
2. in the reader, the databse consumes this log stream by considering each log record in tern
3. if log record refers to a page in the reader's buffer cache, it uses the log applicator to apply the specified redo operation to the page in the cache; otherwise it simply discards the log record
4. replicas consume log records asyncly from the perspective of the writeer, which acknowledge user commits independent of the replica
5. replica rules: a. the only log records that will be applied are those whose LSN is less than or equal to the VDL b. the log records that are part of a single mini-transactino are applied atomically in the replica's cache to ensure that the replica sees a consistent view of all database objects
### III. Recovery
1. periodically checkpoint + redo log
## Putting It All Together

