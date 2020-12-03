# Paper: In Search of an Understandable Consensus Algorithm
## Raft Basics
### I. three parts
1. Leader Election
2. Log Replication
3. Safety
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