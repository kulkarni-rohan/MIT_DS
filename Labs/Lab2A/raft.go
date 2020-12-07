package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import "sync"
import "sync/atomic"
import "../labrpc"
import "time"
import "math/rand"
import "fmt"
// import "bytes"
// import "../labgob"


// status
const (
	LEADER = 0
	CANDIDATE = 1
	FOLLOWER = 2
)

// heartbeat interval in milliseconds
const (
	HEARTBEAT_MIN = 500
	HEARTBEAT_RANGE = 500
	HEARTBEAT_INTERVAL = 100
)

// rpc timeout in milliseconds
const TIMEOUT = 50

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

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	term	  int
	state	  int
	votedFor  int
	cd 		  CountDown
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

}

type Data struct {
	val 	int
	mu	sync.Mutex
}

type CountDown struct {
	version 	int64
	istimeout	bool
	mu			sync.Mutex
	cond		*sync.Cond
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	// Your code here (2A).
	rf.mu.Lock()
	term := rf.term
	isleader := (rf.state == LEADER)
	rf.mu.Unlock()
	return term, isleader
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}


//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term 			int
	CandidateID		int
	// LastLogIndex	int
	// LastLogTerm		int
}

type RequestVoteReply struct {
	// Your data here (2A).
	Term			int
	VoteGranted		bool
}

type AppendEntriesArgs struct {
	Term			int
	LeaderID		int
	// PrevLogIndex	int
	// PrevLogTerm		int
	Entries			[]int
}

type AppendEntriesReply struct {
	Term			int
	Success 		bool
}

// if request term is smaller or equal to current term, success
// upon receiving heartbeat, reset the timer setup in election (conditional variable)
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	fmt.Printf("%v: received AppendEntries from %v in term %v\n",rf.me,args.LeaderID,args.Term)
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if args.Term < rf.term {
		reply.Success = false
		reply.Term = rf.term
		fmt.Printf("%v: AppendEntries from %v failed\n",rf.me,args.LeaderID)
		return
	}
	if len(args.Entries) == 0 {
		// heartbeat
		if args.Term > rf.term {
			rf.term = args.Term
			rf.votedFor = -1
			rf.state = FOLLOWER
		}
		rf.cd.cond.Broadcast()
		reply.Success = true
		reply.Term = rf.term
		fmt.Printf("%v: AppendEntries from %v succeeded\n",rf.me,args.LeaderID)
		return
	}
}

func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	fmt.Printf("%v: received RequestVote from %v\n",rf.me,args.CandidateID)
	if args.Term < rf.term {
		// fail: outdated request
		reply.VoteGranted = false
		reply.Term = rf.term
		fmt.Printf("%v: RequestVote from %v failed\n",rf.me,args.CandidateID)
		return
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if args.Term > rf.term {
		rf.term = args.Term
		rf.state = FOLLOWER
	} else if rf.votedFor != -1 {
		// fail: already voted
		rf.votedFor = args.CandidateID
		reply.VoteGranted = false
		reply.Term = rf.term
		fmt.Printf("%v: RequestVote from %v failed\n",rf.me,args.CandidateID)
		return
	}
	// success
	rf.votedFor = args.CandidateID
	reply.VoteGranted = true
	reply.Term = rf.term
	fmt.Printf("%v: RequestVote from %v succeeded\n",rf.me,args.CandidateID)
	return
}

// if it's in LEADER state, send heartbeats
// if FOLLOWER, start timer for every round, heartbeat can reset this timer
// then timer is up (timeout), it raises an election
func (rf *Raft) election() {
	cd := &rf.cd
	cd.cond.L.Lock()
	for {
		if rf.killed() {
			return
		}
		if rf.state == LEADER {
			rf.heartbeats()
			time.Sleep(HEARTBEAT_INTERVAL*time.Millisecond)
		} else {
			for ; ; cd_ver_inc(cd) {
				go rf.setwakeup(cd)
				cd.cond.Wait()
				cd.mu.Lock()
				if cd.istimeout && rf.launch_election() {
					cd.istimeout = false
					rf.mu.Lock()
					fmt.Printf("%v: Became Leader\n",rf.me)
					rf.state = LEADER
					rf.votedFor = -1
					rf.mu.Unlock()
					cd.mu.Unlock()
					break
				} else {
					rf.state = FOLLOWER
					rf.votedFor = -1
				}
				cd.mu.Unlock()
			}
		}
	}
}

// send every machine heartbeat
// record the highest term meanwhile
// finally, if it fails, it set itself to the most updated term
func (rf *Raft) heartbeats() bool {
	rf.mu.Lock()
	fmt.Printf("%v: heartbeats in term %v\n",rf.me, rf.term)
	defer rf.mu.Unlock()
	args := AppendEntriesArgs{rf.term,rf.me,[]int{}}
	term := Data{rf.term, sync.Mutex{}}
	for i, peer := range rf.peers {
		if i == rf.me {
			continue
		}
		go func(peer *labrpc.ClientEnd) {
			reply := AppendEntriesReply{}
			peer.Call("Raft.AppendEntries", &args, &reply)
			if !reply.Success {
				term.mu.Lock()
				if reply.Term > term.val {
					term.val = reply.Term
				}
				term.mu.Unlock()
			}
		} (peer)
	}
	time.Sleep(TIMEOUT*time.Millisecond)
	if term.val < rf.term {
		rf.term = term.val
		rf.votedFor = -1
		rf.state = FOLLOWER
		return false
	}
	return true
}

// launch election, send every machine RequestVote
// record the highest term meanwhile
// finally, if it fails, it set itself to the most updated term
func (rf *Raft) launch_election() bool {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.state = CANDIDATE
	rf.term++
	rf.votedFor = rf.me
	fmt.Printf("%v: launched election in term %v\n",rf.me, rf.term)
	vote := Data{1,sync.Mutex{}}
	term := Data{rf.term, sync.Mutex{}}
	args := RequestVoteArgs{rf.term, rf.me}
	for i, peer := range rf.peers {
		if i == rf.me {
			continue
		}
		go func(peer *labrpc.ClientEnd) {
			reply := RequestVoteReply{}
			peer.Call("Raft.RequestVote", &args, &reply)
			if reply.VoteGranted {
				vote.mu.Lock()
				vote.val++
				vote.mu.Unlock()
			} else {
				term.mu.Lock()
				if reply.Term > term.val {
					term.val = reply.Term
				}
				term.mu.Unlock()
			}
		} (peer)
	}
	time.Sleep(TIMEOUT*time.Millisecond)
	fmt.Printf("%v: finished election in term %v\n",rf.me, rf.term)
	if term.val > rf.term || vote.val <= len(rf.peers)/2 {
		rf.state = FOLLOWER
		rf.term = term.val
		return false
	}
	return true
}

// reset the istimeout and increment the countdown version atomically
func cd_ver_inc(cd *CountDown) {
	cd.mu.Lock()
	cd.version++
	cd.istimeout = false
	cd.mu.Unlock()
}

// a wakeup timer
func (rf *Raft) setwakeup(cd *CountDown) {
	cd.mu.Lock()
	version := cd.version
	cd.mu.Unlock()
	time.Sleep(time.Duration(randgen()) * time.Millisecond)
	cd.cond.L.Lock()
	cd.mu.Lock()
	if version == cd.version {
		cd.istimeout = true
		cd.cond.Broadcast()
	}
	cd.mu.Unlock()
	cd.cond.L.Unlock()
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).


	return index, term, isLeader
}

//
// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
//
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

// return the randome timeout
func randgen() int {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	return HEARTBEAT_MIN + r1.Intn(HEARTBEAT_RANGE)
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).
	fmt.Printf("%v: initialized\n", rf.me)
	rf.cd = CountDown{0,false,sync.Mutex{},sync.NewCond(&sync.Mutex{})}
	rf.term = 0
	rf.state = FOLLOWER
	rf.votedFor = -1
	go rf.election()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())


	return rf
}
