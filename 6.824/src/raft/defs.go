package raft

// status
const (
	LEADER    = 0
	CANDIDATE = 1
	FOLLOWER  = 2
)

// heartbeat interval in milliseconds
const (
	HEARTBEAT_MIN      = 200
	HEARTBEAT_RANGE    = 400
	HEARTBEAT_INTERVAL = 50
)

// rpc timeout in milliseconds
const TIMEOUT = 20

// agreement heartbeats ratio
const AHR = 20

func Max64(a int64, b int64) int64 {
	if a > b { return a } else { return b }
}
func Min64(a int64, b int64) int64 {
	if a < b { return a } else { return b }
}

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}

type Entry struct {
	Term int
	Cmd  interface{}
}

type RequestVoteArgs struct {
	// Your data here 2A
	Term        int
	CandidateID int
	// Your data here 2B
	LastLogIndex int
	LastLogTerm  int
}

type RequestVoteReply struct {
	// Your data here (2A).
	Term        int
	VoteGranted bool
}

type AppendEntriesArgs struct {
	// Your data here 2A
	Term     int
	LeaderID int
	// Your data here 2B
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []Entry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term    int
	Success bool
}

type InstallSnapshotArgs struct {
	Term		int
	// LeaderId	int
	// LastIncludedIndex	int
	// LastIncludedTerm	int
	// Offset 		int
	LeaderID    int
	State 		[]byte
	Snapshot 	[]byte
	// Done 		bool
}

type InstallSnapshotReply struct {
	Term		int
}