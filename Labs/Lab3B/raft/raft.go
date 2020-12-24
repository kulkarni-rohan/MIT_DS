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

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"../labrpc"
	"bytes"
	"../labgob"
)

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here 2A
	// Persistent
	term     int
	votedFor int
	log      []Entry
	// Volatile
	state int
	cd    CountDown
	// Your data here 2B
	commitIndex int // next record to commit
	nextIndex   []int
	matchIndex  []int
	applyCh     chan ApplyMsg
	// Your data here 2C
	// 3B
	offset int // read index = index + offset
	disgardCh	chan bool
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

}

type Data struct {
	val int
	mu  sync.Mutex
}

type CountDown struct {
	version   int64
	istimeout bool
	mu        sync.Mutex
	cond      *sync.Cond
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	// Your code here (2A).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	term := rf.term
	isleader := (rf.state == LEADER)
	return term, isleader
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
// only can be called when caller is holding rf.mu
func (rf *Raft) persist() {
	// Your code here (2C)
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.votedFor)
	e.Encode(rf.term)
	e.Encode(rf.offset)
	nLog := len(rf.log)
	e.Encode(nLog)
	for i := 0; i < nLog; i++ {
		entry := rf.log[i]
		e.Encode(entry)
	}
	data := w.Bytes()
	rf.persister.SaveRaftState(data)
	return
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	// Your code here (2C).
	// if no error Decode returns error
	fmt.Printf("%v: readPersist\n", rf.me)
	if data == nil || len(data) < 1 {
		rf.votedFor = -1
		rf.term = 0
		rf.log = []Entry{Entry{-1, 100}}
		return
	}
	rf.log = []Entry{}
	d := labgob.NewDecoder(bytes.NewBuffer(data))
	var votedFor, term, offset, nLog int
	d.Decode(&votedFor)
	d.Decode(&term)
	d.Decode(&offset)
	rf.votedFor, rf.term, rf.offset = votedFor, term, offset
	rf.commitIndex = rf.offset
	d.Decode(&nLog)
	for i := 0; i < nLog; i++ {
		var entry Entry
		d.Decode(&entry)
		rf.log = append(rf.log, entry)
	}
	return
}

// discard logs before idx, after this operation, idx becomes index 1
// cidx -> raft index; tidx -> server index
func (rf *Raft) DiscardBefore(tidx int, snapshot []byte) {
	// 3B
	if tidx == -1 {
		rf.disgardCh <- false
		return
	}
	fmt.Printf("%v: DiscardBefore %v\n", rf.me, tidx)
	cidx := tidx - rf.offset
	if cidx < 1 {
		fmt.Printf("%v: cidx < 1\n", rf.me)
		return
	}
	if cidx <= len(rf.log) {
		rf.log = rf.log[cidx-1:]
	} else {
		panic("raft.DiscardBefore")
	}
	rf.offset += cidx - 1
	
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.votedFor)
	e.Encode(rf.term)
	e.Encode(rf.offset)
	nLog := len(rf.log)
	e.Encode(nLog)
	for i := 0; i < nLog; i++ {
		entry := rf.log[i]
		e.Encode(entry)
	}
	state := w.Bytes()

	rf.persister.SaveStateAndSnapshot(state, snapshot)
	rf.disgardCh <- true
}

func (rf *Raft) StateSize() int {
	return rf.persister.RaftStateSize()
}

// if request term is smaller or equal to current term, success
// reset the timer setup in election (conditional variable)
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if args.Term < rf.term {
		reply.Success = false
		reply.Term = rf.term
		fmt.Printf(
			"%v: AppendEntries from %v failed: term{%v %v}, {%v %v}, nEntry: %v\n",
			rf.me, args.LeaderID,
			args.Term, rf.term,
			args.PrevLogIndex, args.PrevLogTerm,
			len(args.Entries),
		)
		return
	} else if args.Term > rf.term {
		rf.term = args.Term
		rf.votedFor = -1
		rf.persist()
		rf.state = FOLLOWER
	}
	rf.cd.cond.Broadcast()
	reply.Success = true
	reply.Term = rf.term

	cLog := len(rf.log) // current # of Log 
	tLog := len(rf.log) + rf.offset // total # of Log 
	if !(len(args.Entries) == 0 && rf.commitIndex == args.LeaderCommit &&
	args.PrevLogTerm == rf.log[cLog-1].Term && args.PrevLogIndex == tLog-1) {
		fmt.Printf(
			"%v: AppendEntries from %v in term %v: "+
				"pidx: {%v %v}, pterm: {%v %v}, commit: {%v %v}, nEntry: %v\n",
			rf.me, args.LeaderID, args.Term,
			args.PrevLogIndex, tLog-1,
			args.PrevLogTerm, rf.log[cLog-1].Term,
			args.LeaderCommit, rf.commitIndex, 
			len(args.Entries),
		)
	}
	if args.PrevLogIndex > tLog - 1 || args.PrevLogTerm > rf.log[cLog-1].Term {
		// outdated log
		reply.Success = false
		reply.Term = rf.term
		return
	} else {
		// heartbeat / update
		nEntry := len(args.Entries)
		pidx := args.PrevLogIndex
		if pidx + nEntry >= rf.offset {
			if pidx + 1 <= rf.offset {
				rf.log = args.Entries[rf.offset-(pidx+1):]
			} else {
				rf.log = append(rf.log[:pidx+1-rf.offset], args.Entries...)
			}
			rf.persist()
		} else {
			reply.Success = false
			reply.Term = rf.term
			return
		}
		tLog := len(rf.log) + rf.offset
		for ; rf.commitIndex < args.LeaderCommit; rf.commitIndex++ {
			if tLog <= rf.commitIndex + 1 {
				break
			}
			rf.applyCh <- ApplyMsg{true, rf.log[rf.commitIndex+1-rf.offset].Cmd, rf.commitIndex+1}
			<-rf.disgardCh
		}
	}
	reply.Success = true
	reply.Term = rf.term
	return
}

// this also reset the timer
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	fmt.Printf(
		"%v: RequestVote from %v: {%v %v}, term: {%v %v}\n",
		rf.me, args.CandidateID,
		args.LastLogIndex, args.LastLogTerm,
		args.Term, rf.term,
	)
	cLog := len(rf.log)
	tLog := len(rf.log) + rf.offset
	if args.Term < rf.term ||
		args.LastLogTerm < rf.log[cLog-1].Term ||
		(args.LastLogTerm == rf.log[cLog-1].Term && args.LastLogIndex < tLog) {
		// fail: outdated request
		reply.VoteGranted = false
		reply.Term = rf.term
		return
	}
	if args.Term > rf.term {
		rf.term = args.Term
		rf.persist()
		rf.state = FOLLOWER
	} else if rf.votedFor != -1 {
		// fail: already voted
		rf.votedFor = args.CandidateID
		rf.persist()
		reply.VoteGranted = false
		reply.Term = rf.term
		return
	}
	// success
	rf.cd.cond.Broadcast()
	rf.votedFor = args.CandidateID
	rf.persist()
	reply.VoteGranted = true
	reply.Term = rf.term
	return
}

func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if args.Term < rf.term {
		reply.Term = args.Term
		return
	}
	fmt.Printf("%v: InstallSnapshot from %v\n", rf.me, args.LeaderID)
	rf.persister.SaveStateAndSnapshot(args.State, args.Snapshot)
	rf.readPersist(rf.persister.ReadRaftState())
	rf.applyCh <- ApplyMsg{false, nil, 0}
}

// if it's in LEADER state, send heartbeats
// if FOLLOWER, start timer for every round, heartbeat can reset this timer
// then timer is up (timeout), it raises an election
// once it becomes LEADER, force all other servers to replicate its log
func (rf *Raft) election() {
	cd := &rf.cd
	cd.cond.L.Lock()
	c := 0
	for {
		if rf.killed() {
			return
		}
		rf.mu.Lock()
		state := rf.state
		rf.mu.Unlock()
		if state == LEADER {
			if c % AHR == 0 {
				rf.mu.Lock()
				cLog := len(rf.log)
				tLog := len(rf.log) + rf.offset
				if cLog > 1 && tLog - 1 > rf.commitIndex {
					// force newly joined nodes to update
					go rf.agreement(tLog-1, rf.log[cLog-1].Cmd)
				} else {
					rf.mu.Unlock()
				}
			} else {
				rf.heartbeats()
			}
			c++
			time.Sleep(HEARTBEAT_INTERVAL * time.Millisecond)
		} else {
			for ; ; cd_ver_inc(cd) {
				go rf.setwakeup(cd)
				cd.cond.Wait()
				cd.mu.Lock()
				if cd.istimeout && rf.launch_election() {
					cd.istimeout = false
					rf.mu.Lock()
					// fmt.Printf("%v: Became Leader in term %v\n",rf.me, rf.term)
					fmt.Printf("\033[0;33m%v: Became Leader in term %v\033[0m\n", rf.me, rf.term)
					rf.state = LEADER
					rf.votedFor = -1
					rf.persist()
					cLog := len(rf.log)
					tLog := len(rf.log) + rf.offset
					if cLog > 1 && tLog - 1 > rf.commitIndex {
						// force follower to replicate its log
						go rf.agreement(tLog - 1, rf.log[cLog-1].Cmd)
					} else {
						rf.mu.Unlock()
					}
					cd.mu.Unlock()
					break
				} else {
					rf.mu.Lock()
					rf.state = FOLLOWER
					rf.mu.Unlock()
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
	// fmt.Printf("%v: heartbeats in term %v\n",rf.me, rf.term)
	defer rf.mu.Unlock()
	cLog := len(rf.log)
	tLog := len(rf.log) + rf.offset
	args := AppendEntriesArgs{
		rf.term, rf.me,
		tLog - 1, rf.log[cLog-1].Term,
		[]Entry{}, rf.commitIndex,
	}
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
		}(peer)
	}
	time.Sleep(TIMEOUT * time.Millisecond)
	if term.val > rf.term {
		rf.term = term.val
		rf.votedFor = -1
		rf.persist()
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
	rf.persist()
	// fmt.Printf("%v: launched election in term %v\n",rf.me, rf.term)
	vote := Data{1, sync.Mutex{}}
	term := Data{rf.term, sync.Mutex{}}
	cLog := len(rf.log)
	tLog := len(rf.log) + rf.offset
	args := RequestVoteArgs{rf.term, rf.me, tLog, rf.log[cLog-1].Term}
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
		}(peer)
	}
	time.Sleep(TIMEOUT * time.Millisecond)
	term.mu.Lock()
	max_term := term.val
	term.mu.Unlock()
	vote.mu.Lock()
	vote_cnt := vote.val
	vote.mu.Unlock()
	if max_term > rf.term || vote_cnt <= len(rf.peers)/2 {
		rf.state = FOLLOWER
		rf.term = max_term
		rf.persist()
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

// a wakeup timer, each time generates a different ElectionTimeout
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
	rf.mu.Lock()
	// Your code here (2B).
	if rf.state == LEADER {
		tLog := len(rf.log) + rf.offset
		go rf.agreement(tLog, command)
		return tLog, rf.term, true
	} else {
		rf.mu.Unlock()
		return -1, -1, false
	}
}

// 1. this defer ensures commitIndex is assigned uniquely to different
// commands, it makes (Start + agreement) atomic, at the same time
// Start() can return immediately
// 2. if recipient's log is outdated, it simply went back an index and 
// resend
func (rf *Raft) agreement(idx int, command interface{}) bool {
	defer rf.mu.Unlock()
	entry := Entry{rf.term, command}
	tLog := len(rf.log) + rf.offset
	fmt.Printf(
		// "%v: agreement[%v] %v in term %v\n", 
		"\033[1;33m%v: agreement[%v] %v in term %v\033[0m\n", 
		rf.me, idx, command, rf.term,
	)
	if idx == tLog {
		rf.log = append(rf.log, entry)
		rf.persist()
	}
	nAppended := Data{1, sync.Mutex{}}
	for i, peer := range rf.peers {
		if i == rf.me {
			continue
		}
		args := AppendEntriesArgs{
			rf.term, rf.me, idx - 1, rf.log[idx-1-rf.offset].Term,
			[]Entry{entry}, rf.commitIndex,
		}
		go func(peer *labrpc.ClientEnd) {
			for {
				reply := AppendEntriesReply{}
				if !peer.Call("Raft.AppendEntries", &args, &reply) {
					return
				}
				if reply.Success {
					nAppended.mu.Lock()
					nAppended.val++
					nAppended.mu.Unlock()
					return
				} else if !(reply.Term > args.Term) && args.PrevLogIndex - rf.offset > 0 {
					i := args.PrevLogIndex - rf.offset - 1
					for ok := false; !ok && i > 0; i-- {
						ok = rf.log[i].Term < rf.term && rf.log[i-1].Term < rf.log[i].Term
					}
					args.PrevLogIndex = i + rf.offset
					args.PrevLogTerm = rf.log[i].Term
					args.Entries = rf.log[i+1:]
				} else if !(reply.Term > args.Term) {
					state := rf.persister.ReadRaftState()
					snapshot := rf.persister.ReadSnapshot()
					args1 := InstallSnapshotArgs{rf.term, rf.me, state, snapshot}
					reply1 := InstallSnapshotReply{}
					peer.Call("Raft.InstallSnapshot", &args1, &reply1)
					return
				} else {
					return
				}
			}
		} (peer)
	}
	time.Sleep(TIMEOUT * time.Millisecond)
	nAppended.mu.Lock()
	nAppend := nAppended.val
	nAppended.mu.Unlock()
	if nAppend > len(rf.peers)/2 && idx >= rf.commitIndex + 1 {
		if idx == rf.commitIndex + 1 {
			rf.applyCh <- ApplyMsg{true, rf.log[idx-rf.offset].Cmd, idx}
			<-rf.disgardCh
		} else {
			for i := rf.commitIndex + 1; i <= idx; i++ {
				rf.applyCh <- ApplyMsg{true, rf.log[i-rf.offset].Cmd, i}
				<-rf.disgardCh
			}
		}
		rf.commitIndex = idx
		return true
	} else {
		return false
	}
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
	fmt.Printf("\033[1;31m%v: initialized\033[0m\n", rf.me)
	// fmt.Printf("%v: initialized\n", rf.me)
	rf.cd = CountDown{0, false, sync.Mutex{}, sync.NewCond(&sync.Mutex{})}
	rf.applyCh = applyCh
	rf.disgardCh = make(chan bool)
	rf.state = FOLLOWER
	rf.commitIndex = 0
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	go rf.election()

	return rf
}
