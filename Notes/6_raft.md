# Paper: In Search of an Understandable Consensus Algorithm
## Raft Basics
### I. three parts
1. Leader Election
2. Log Replication
3. Safety
### II. properties
1. Election Safety: at most one leader can be elected in a given term
2. Leader Append-Only: a leader never overwrites or deletes entries in its log; it only appends new entries
3. Log Matching: if two logs contain an entry with the same index and term, then the logs are identical in all entries up through the given index
4. Leader Completeness: if a log entry is committed in a given term, then that entry will be present in the logs of the leaders for all higher-numbered terms
5. State Machine Safety: If a server has applied a log entry at a given index to its state machine, no other server will ever apply different log entry for the same index
## Leader Election
1. when servers start up, they began as followers
2. leader sends periodic heartbeats (AppendEntries RPC with no log entries) to all followers to maintain authority.
3. if follower receives no communication over `election timeout`, then it assumes there is no viable leader and begins an election to choose a new leader.
4. begin an election: follower increments its current term and transitions to candidate state, vote for itself and issue RequestVoteRPCs in parallel to each else
5. candidate continue in this state until one of three things happens: a. it wins b. other wins c. no one wins
6. candidate wins if it receives the majority of votes, voting is first-come-first-served. 
7. while waiting for votes, a candidate may receive an AppendEntries RPC from another server claiming to be leader. If its term is larger or equal than itself's, the candidate recognizes the leader as legit and returns to follower state
8. no one wins -> after timeout, new election (150ms - 300ms)
## Log Replication
### I. steps
1. leader appends log entry in itself
2. send AppendEntry to every other server
3. if receives negative reply, retry indefinitely
### II. commit
1. log entry is committed once the leader that created the entry has replicated it on a majority of the servers. This also commits all preceding entries in the leader's log, including entries created by previous leaders
2. leader keep tracks of the highest index it knows to be committed, and it includes that index in future AppendEntries RPCs so that other servers will eventually find out
### III. inconsistency handling
forcing the followers' logs to deplicate its own
### IV. nextIndex
1. leader maintains a `nextIndex` fields for all followers.
2. it initializes `nextIndex` values to the index just after the last one in its log.
3. if follower's log is inconsistent with the leader's, the AppendEntries RPC consistency check will fail in the next AppendEntries RPC. 
4. After a rejection, the leader decrements nextIndex and retries the AppendEntries RPC
## Safety
goal: ensure follower to execute same command in same order
### I. Election Restriction
1. only elected as leader when candidate's log is at least as up-to-date as any other log in that majority
2. determine which of two logs is more up-to-date by comparing the index and term of the last entries in the logs.
### II. Committing entries from previous terms
1. never commits log entries from previous terms by counting replicas.
2. only log entries from the leader's current term are committed by counting replicas
3. once an entry from the current term has been committed in this way, then all prior entries are committed indirectly because of the Log Matching Property.
### III. Safety Argument
Because both commit and election will require agreement from majority of servers, they must overlap. This ensures part II measurements are correct.
## Follower and Candidate Crashes
1. future RequestVote and AppendEntries send to it will fail. Raft handles this failures by retrying indefinitely.
2. if server completes RPC but fails to respond, it's no harm because Raft RPCs are idempotent
## Timing and Availability
broadcastTime << electionTimeout << MTBF
1. broadcastTime: average time it takes a server to send RPCs in parallel to every server in the cluster and receive their responese, 0.5ms to 20ms
2. electionTimeout: 10ms to 500ms
3. MTBF: average time between failures for a single server, several months or more.
## Log Compaction
### I. InstallSnapshot RPC
1. reply immediately if term < currentTerm
2. create new snapshot file if first chunk (offset is 0)
3. write data into snapshot file at given offset
4. reply and wait for mroe data chunks if done is false
5. save snapshot file, discard any exisiting or partial snapshot with a smaller index
6. if existing log entry has same index and term as snapshot's last included entry, retain log entries following it and reply 
7. discard the entire log
8. reset state machine using snapshot contents (and load snapshot's cluster configuration)
## Client Interaction
### I. starts up
1. when starts up, it connects to a randomly chosen server, if not leader, it rejects client's request and supply information about the most recent leader it has heard from
2. if leader crashes, client requests will time out, client then try again with randomly-chosen servers
### II. command serial number
1. assign unique serial numbers to every command
2. state machine tracks the latest serial number
3. if it receives a command whose serial number has already been executed, it replies immediately without re-executing
### III. prevent reading stale data
1. a leader must have the latest information on which entries are committed
2. a leader must check whether it has been deposed before processing a read-only requests. Raft handles this by having the leader exchange heartbeat messages with a majority of the cluster before responding to read-only requests.
