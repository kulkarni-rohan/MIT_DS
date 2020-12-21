# Paper: Spanner: Google's Globally-Distributed Database
## Implementation
0. FIGURE 1
1. universe: a spanner deployment
2. zone: Spanner is organized as a set of zones, where each zone is the rough analog of a deployment of Bigtable servers. Zones are the unit of administrative deployment, also the unit of physical isolation
3. a zone has one zonemaster and between one hundred and several thousand spanservers
4. zonemaster assigns data to spanservers
5. spanservers serve data to clients
6. per=zone location proxies are used by clients to locate the spanservers assigned to serve their data
7. the universe master and the placement driver are currently singletons
8. universe master  is a console that displays status info about all the zones for interactive debugging.
9. the placement driver handles automated movement of data across zones on the timescale of minutes
### I. Spanserver Software Stack
0. FIGURE 2
1. each spanserver is responsible for between 100 and 1000 instances of a data structure called a `tablet`
2. {key: string, timestamp: int64} -> string
3. writes must initiate the Paxos protocol at the leader; reads access state directly from the dunderlying tablet at any replica that is sufficiently up-to-date
4. lock table is used to implement concurrency control, it contains the state for two-phase locking: it maps ranges of keys to lock states
5. each spanserver also implements a transaction manager to support distributed transactions. The transaction manager is used to implement a participant leader; the other replicas in the group will be referred to as participant slaves
6. one of the participant groups is chosen as tthe coordinator: the participant leader of that group will be referred to as the coordinator leader, and the slaves of that group as coordinator slaves
### II. Directories and Placement
1. a directory is the unit of data placement, also the smallest unit whose placement can be specified by an application
2. move dir is the background task used to move directories between Paxos groups. Also used to add/remove replicas to Paxos groups. Movedir registers the fact that it is starting to move data and moves the data in the background. When it moved all but a nominal amount of the data, it uses a transaction to atomically move that nominal ammount and update the metadata for the two Paxos groups
3. An application controls how data is replicated by tagging each database and/or individual directories witha combination of those options
4. Spanner will shard a directory into multiple fragments if it grows too large
### III. Data Model
1. support synchronous replication across datacenters.
2. an application creates one or more databases in a universe. Each database can contain an unlimited number of schematized tables. Every table is required to have an ordered set of one or more primary-key columns 
3. every Spanner databse must be partitioned by clients into one or more hierarchies of tables. Declared via the INTERLEAVE IN declarations
## TrueTime
1. TT.Now() -> TTinterval: \[earliest, latest\]
2. TT.after(t) -> true if t has definetely passed
3. TT.before(t) -> true if t has definitely not arrived
## Concurrency Control
### I. Timestamp Management
1. support: read-write transactions, read-only transactions, and snapshot reads
2. reads in a read-only transaction execute at a system-chosen timestamp without locking, so the incoming writes are not blocked.
3. snapshot read is a read in the past that executes without locking. A client can either specify a timestamp for a snapshot read, or provide an upper bound on the desired timestamp's staleness and let Spanner choose a timestamp
4. for both read-only transactions and snapshot reads, commit is inevitable once a timestamp has been chosen, unless the data at that timestamp has been garbage collected. As a result, clients can avoid buffering results inside a retry loop. When a server fails, clients can internally continue the query on a different server by repeating the timestamp and the current read position
#### A. Paxos Leader Leases
1. timed lease votes
2. lease interval starts when it discovers it has a quorum of lease votes, and ends when it no longer has a qquorum of lease votes
3. disjointness invariant: for each Paxos group, each Paxos leader's lease interval is disjoint from every other leader's
4. Smax -> maximum timestamp used by a leader, before abdicating, a leader must wait until TT.after(Smax) is true
#### B. Assigning Timestamps to RW Transactions
1. monotonicity invariant: within each Paxos group, Spanner assigns timestamps to Paxos writes in monotonically increasing order, even across leaders. A single leader replica can trivially assign timestamps in monotonically increasing order.
2. external consistency invariant: if the start of a transaction T2 occurs after the commit of a transaction T1, then the commit timestamp of T2 must be greater than the commit timestamp of T1.
3. Start, Commit Wait: see paper page 7
#### C. Serving Reads at a Timestamp
1. tsafe = min(t^Paxos_safe, t^TM_safe)
2. Paxos safe time: highest-applied Paxos write
3. t^TM_safe is infinity at a replica if there are 0 prepared transactions
4. if there are transactions in between the two phases of two-phase commit. Because it doesn't know whether such transactions will commit
5. every participant leader for a transaction Ti assigns a prepare timestamp s^prepare_i,g to its prepare record. t^TM_safe = mini(s^prepare_i,g) - 1
6. safe time is used to determine if a replica's state is sufficiently up-to-date to satisfy a read. A replica can satisfy a read at a timestamp t if t <= tsafe
#### D. Assigning Timestamps to RO Transactions
1. a read-only transaction executes in two phases: assign a timestamp sread, and then execute the transaction's reads as snapshot reads at sread.
2. sread = TT.now().latest
3. to reduce the chance of blocking, Spanner should assign the oldest timestamp that preserves external consistency
### II. Details
#### A. Read-Write Transactions
1. writes that occur in a transaction are buffered at the client until commit. 
2. reads in a transactino do not see the effects of the transaction's writes
3. coordinator leader waits until TT.after(s)
#### B. Read-Only Transactions
1. assigning a timestamp requires a negotiation phase between all of the Paxos groups that are involved in the read. As a result, Spanner reqyures a scope expression for every read-only transaction
2. if the scope's values are served by a single Paxos group, then the client issues the read-only transaction to that group's leader.
3. the leader assigns sread and executes the read.
4. For a single-site read, Spanner generally does better than TT.now().latest.
5. LastTS() -> timestamp of the last committed write at a Paxos group. If no prepared transactinos, the assignment sread = LastTS() trivially satisfies external consistency.   
#### C. Schema-Change Transactions
non-blocking variant of a standard transaction
#### D. Refinements
...