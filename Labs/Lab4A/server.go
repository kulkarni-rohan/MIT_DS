package shardmaster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"../labgob"
	"../labrpc"
	"../raft"
)

type cond struct {
	sync.Mutex
	cond *sync.Cond
}

type ck struct {
	id      int64
	ver     int64
	records map[int64]interface{}
	// records []interface{}
	mu sync.Mutex
}

type ShardMaster struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg

	// Your data here.
	lastcommit   int           // last commit index
	conds        []*cond       // condition variables
	cks          []ck          // clerks
	ckId2Idx     map[int64]int // clerk Id to Index in clerks array
	cksMu        sync.Mutex    // mutex for clerks
	persister    *raft.Persister
	maxraftstate int

	configs   []Config // indexed by config num
	configVer int
}

type Op struct {
	// Your data here.
	Id   int64
	Ver  int64
	Type string
	// Join
	Servers map[int][]string
	// Leave
	GIDs []int
	// Move
	Shard int
	GID   int
	// Query
	Num int
}

func newCond() *cond {
	c := cond{}
	c.cond = sync.NewCond(&c)
	return &c
}

// extend condition variable to be at least as long as the client number
func (sm *ShardMaster) extendconds(idx int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for len(sm.conds) <= idx {
		sm.conds = append(sm.conds, newCond())
	}
}

// wait at idx
func (sm *ShardMaster) wait(idx int) {
	sm.extendconds(idx)
	sm.conds[idx].Lock()
	sm.conds[idx].cond.Wait()
	sm.conds[idx].Unlock()
}

// wakeup threads at idx
func (sm *ShardMaster) wakeup(idx int) {
	sm.extendconds(idx)
	sm.conds[idx].cond.Broadcast()
}

// update the ck committed records
func (sm *ShardMaster) updateCkRecords(id int64, ver int64, reply interface{}) {
	idx := sm.ckId2Idx[id]
	ck := &sm.cks[idx]
	// fmt.Printf("SM Server %v: id: %v, ver: %v, ck.ver: %v\n", sm.me, ck.id, ver, ck.ver)
	if ver != ck.ver+1 {
		return
		// panic("sm.updateCkRecords: not continuous")
	}
	ck.ver++
	ck.records[ver] = reply
}

// must be called with sm.cksMu held
func (sm *ShardMaster) createRecord(id int64, ver int64) {
	nCks := len(sm.cks)
	sm.cks = append(sm.cks, ck{id, ver, make(map[int64]interface{}), sync.Mutex{}})
	sm.ckId2Idx[id] = nCks
}

// if the ck does not exist, create it
// if exist, check if the records are continuous, if not, return an Error
// to let client wait
func (sm *ShardMaster) checkContinuity(id int64, ver int64) bool {
	sm.cksMu.Lock()
	defer sm.cksMu.Unlock()
	idx, ok := sm.ckId2Idx[id]
	if !ok {
		sm.createRecord(id, 0)
		return true
	}
	ck := &sm.cks[idx]
	res := ver <= ck.ver+1
	return res
}

// check check if the version of the opration is executed
// if exist, return it
func (sm *ShardMaster) checkSubmitted(id int64, ver int64) (interface{}, bool) {
	sm.cksMu.Lock()
	defer sm.cksMu.Unlock()
	idx := sm.ckId2Idx[id]
	ck := &sm.cks[idx]
	record := ck.records[ver]
	if ck.ver >= ver {
		// fmt.Printf("SM Server %v: Loading, ver: %v, ck.ver: %v\n", sm.me, ver, ck.ver)
		return record, true
	}
	return 1, false
}

func (sm *ShardMaster) submit(cmd Op) int {
	lastcommit := sm.lastcommit
	idx, _, isLeader := sm.rf.Start(cmd)
	if !isLeader {
		return 0
	}
	if idx <= lastcommit {
		return 0
		// panic("sm.submit: outdated commitindex")
	}
	return idx
}

func (sm *ShardMaster) Join(args *JoinArgs, reply *JoinReply) {
	// Your code here.
	if !sm.checkContinuity(args.Id, args.Ver) {
		reply.Err = ErrNoContinuity
		return
	}
	_, isSubmitted := sm.checkSubmitted(args.Id, args.Ver)
	if isSubmitted {
		return
	}
	cmd := Op{args.Id, args.Ver, JOIN, args.Servers, nil, -1, -1, -1}
	idx := sm.submit(cmd)
	if idx == 0 {
		reply.WrongLeader = true
		return
	}
	sm.wait(idx)
}

func (sm *ShardMaster) Leave(args *LeaveArgs, reply *LeaveReply) {
	// Your code here.
	if !sm.checkContinuity(args.Id, args.Ver) {
		reply.Err = ErrNoContinuity
		return
	}
	_, isSubmitted := sm.checkSubmitted(args.Id, args.Ver)
	if isSubmitted {
		return
	}
	cmd := Op{args.Id, args.Ver, LEAVE, nil, args.GIDs, -1, -1, -1}
	idx := sm.submit(cmd)
	if idx == 0 {
		reply.WrongLeader = true
		return
	}
	sm.wait(idx)
}

func (sm *ShardMaster) Move(args *MoveArgs, reply *MoveReply) {
	// Your code here.
	if !sm.checkContinuity(args.Id, args.Ver) {
		reply.Err = ErrNoContinuity
		return
	}
	_, isSubmitted := sm.checkSubmitted(args.Id, args.Ver)
	if isSubmitted {
		return
	}
	cmd := Op{args.Id, args.Ver, MOVE, nil, nil, args.Shard, args.GID, -1}
	idx := sm.submit(cmd)
	if idx == 0 {
		reply.WrongLeader = true
		reply.Err = ErrWrongLeader
		return
	}
	sm.wait(idx)
}

func (sm *ShardMaster) Query(args *QueryArgs, reply *QueryReply) {
	// Your code here.
	if !sm.checkContinuity(args.Id, args.Ver) {
		reply.Err = ErrNoContinuity
		return
	}
	oldReply, isSubmitted := sm.checkSubmitted(args.Id, args.Ver)
	if oldReply == nil {
		sm.mu.Lock()
		nConfig := sm.configVer
		sm.mu.Unlock()
		if args.Num >= nConfig || args.Num < 0 {
			reply.Config = sm.configs[nConfig-1]
		} else {
			reply.Config = sm.configs[args.Num]
		}
		return
	}
	if isSubmitted {
		reply.Config = oldReply.(QueryReply).Config
		reply.Err = oldReply.(QueryReply).Err
		reply.WrongLeader = oldReply.(QueryReply).WrongLeader
		fmt.Printf("SM Server %v: return submitted: %v %v\n", sm.me, args, reply)
		return
	}
	cmd := Op{args.Id, args.Ver, QUERY, nil, nil, -1, -1, args.Num}
	idx := sm.submit(cmd)
	if idx == 0 {
		reply.WrongLeader = true
		return
	}
	sm.wait(idx)
	oldReply, isSubmitted = sm.checkSubmitted(args.Id, args.Ver)
	if oldReply == nil {
		sm.mu.Lock()
		nConfig := sm.configVer
		sm.mu.Unlock()
		if args.Num >= nConfig || args.Num < 0 {
			reply.Config = sm.configs[nConfig-1]
		} else {
			reply.Config = sm.configs[args.Num]
		}
		return
	}
	if isSubmitted {
		reply.Config = oldReply.(QueryReply).Config
		reply.Err = oldReply.(QueryReply).Err
		reply.WrongLeader = oldReply.(QueryReply).WrongLeader
		fmt.Printf("SM Server %v: return submitted: %v %v\n", sm.me, args, reply)
		return
	} else {
		// panic("sm.Get: submitted but cannot find")
		reply.Err = ErrUnknown
		return
	}
}

//
// the tester calls Kill() when a ShardMaster instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (sm *ShardMaster) Kill() {
	sm.rf.Kill()
	// Your code here, if desired.
}

func inArray(e int, b []int) bool {
	for _, v := range b {
		if e == v {
			return true
		}
	}
	return false
}

func (sm *ShardMaster) lastConfig() Config {
	// DEEPCOPY...
	nConfig := len(sm.configs)
	src := sm.configs[nConfig-1]
	dst := Config{}
	byt, _ := json.Marshal(src)
	json.Unmarshal(byt, &dst)
	return dst
}

func (sm *ShardMaster) execute_join(args JoinArgs) JoinReply {
	lastConfig := sm.lastConfig()
	toDel := []int{}
	nOldGroups := len(lastConfig.Groups)
	for GID, servers := range args.Servers {
		_, ok := lastConfig.Groups[GID]
		if ok {
			toDel = append(toDel, GID)
		} else {
			lastConfig.Groups[GID] = servers
		}
	}
	// remove those keys we already have
	for _, GID := range toDel {
		delete(args.Servers, GID)
	}
	nToAdd := len(args.Servers)
	// the final # of shards per group owns must lie between MIN and MAX
	if nToAdd > 0 {
		lastConfig.Num++
		nGroups := nToAdd + nOldGroups
		MIN := NShards / nGroups
		MAX := (NShards + nGroups - 1) / nGroups
		GID2SHARDS := make(map[int][]int)
		for GID, _ := range lastConfig.Groups {
			GID2SHARDS[GID] = []int{}
		}
		for i := 0; i < NShards; i++ {
			GID := lastConfig.Shards[i]
			GID2SHARDS[GID] = append(GID2SHARDS[GID], i)
		}
		for GID, _ := range args.Servers {
			nAdded := 0
			for GID2, shards := range GID2SHARDS {
				MAX2 := MAX
				if GID2 == 0 {
					MAX2 = 0
				}
				i := 0
				for ; len(shards)-i > MAX2 && nAdded < MIN; i, nAdded = i+1, nAdded+1 {
					lastConfig.Shards[shards[i]] = GID
				}
				GID2SHARDS[GID2] = shards[i:]
			}
			for GID2, shards := range GID2SHARDS {
				MIN2 := MIN
				if GID2 == 0 {
					MIN2 = 0
				}
				if len(shards) > MIN2 && nAdded < MIN {
					nAdded++
					lastConfig.Shards[shards[0]] = GID
					GID2SHARDS[GID2] = shards[1:]
				}
			}
		}
		for GID, shards := range GID2SHARDS {
			for GID2, shards2 := range GID2SHARDS {
				if GID != GID2 && GID != 0 && GID2 != 0 {
					if len(shards) > MAX && len(shards2) < MAX {
						lastConfig.Shards[shards[0]] = GID2
						shards2 = append(shards2, shards[0])
						GID2SHARDS[GID] = shards[1:]
						GID2SHARDS[GID2] = shards2
					}
				}
			}
		}
		fmt.Printf(
			"%v: %v, MIN %v, MAX %v, %v %v\n", 
			sm.me, lastConfig, MIN, MAX, nToAdd, nOldGroups, 
		)
		sm.configs = append(sm.configs, lastConfig)
		sm.configVer++
	}
	return JoinReply{}
}

func (sm *ShardMaster) execute_leave(args LeaveArgs) LeaveReply {
	lastConfig := sm.lastConfig()
	toDel := []int{}
	nOldGroups := len(lastConfig.Groups)
	for _, GID := range args.GIDs {
		_, ok := lastConfig.Groups[GID]
		if ok {
			toDel = append(toDel, GID)
			delete(lastConfig.Groups, GID)
		}
	}
	toArrange := []int{}
	GID2SHARDS_CNT := make(map[int]int)
	for GID, _ := range lastConfig.Groups {
		GID2SHARDS_CNT[GID] = 0
	}
	for i := 0; i < NShards; i++ {
		if inArray(lastConfig.Shards[i], toDel) {
			toArrange = append(toArrange, i)
		} else {
			GID2SHARDS_CNT[lastConfig.Shards[i]]++
		}
	}
	nToArrange := len(toArrange)
	nToDel := len(toDel)
	// the final # of shards per group owns must lie between MIN and MAX
	if nToDel > 0 {
		lastConfig.Num++
		nGroups := nOldGroups - nToDel
		if nGroups <= 0 {
			lastConfig.Shards = [10]int{}
			sm.configs = append(sm.configs, lastConfig)
			return LeaveReply{}
		}
		MIN := NShards / nGroups
		MAX := (NShards + nGroups - 1) / nGroups
		fmt.Printf(
			"SM Server %v: LEAVE, toDel %v, toArrange: %v, MIN: %v, MAX: %v, config %v, G2S %v\n",
			sm.me, toDel, toArrange, MIN, MAX, lastConfig, GID2SHARDS_CNT,
		)
		i := 0
		for GID2, shards_cnt := range GID2SHARDS_CNT {
			for ; shards_cnt < MIN; i, shards_cnt = i+1, shards_cnt+1 {
				lastConfig.Shards[toArrange[i]] = GID2
			}
			GID2SHARDS_CNT[GID2] = shards_cnt
		}
		for GID2, shards_cnt := range GID2SHARDS_CNT {
			if shards_cnt < MAX && i < nToArrange {
				lastConfig.Shards[toArrange[i]] = GID2
				i, shards_cnt = i+1, shards_cnt+1
			}
			GID2SHARDS_CNT[GID2] = shards_cnt
		}
		sm.configs = append(sm.configs, lastConfig)
		sm.configVer++
		fmt.Printf("SM Server %v: config %v after LEAVE %v\n", sm.me, lastConfig, args)
	}
	return LeaveReply{}
}

func (sm *ShardMaster) execute_move(args MoveArgs) MoveReply {
	lastConfig := sm.lastConfig()
	if lastConfig.Shards[args.Shard] != args.GID {
		lastConfig.Num++
		lastConfig.Shards[args.Shard] = args.GID
	}
	sm.configs = append(sm.configs, lastConfig)
	return MoveReply{}
}

// execute operation
func (sm *ShardMaster) execute(op Op) {
	// fmt.Printf("SM Server %v: execute %v\n", sm.me, op)
	idx := sm.ckId2Idx[op.Id]
	ck := &sm.cks[idx]
	ck.mu.Lock()
	defer ck.mu.Unlock()
	_, ok := sm.checkSubmitted(op.Id, op.Ver)
	if ok {
		return
	}
	if op.Type == JOIN {
		args := JoinArgs{op.Servers, -1, -1}
		reply := sm.execute_join(args)
		sm.updateCkRecords(op.Id, op.Ver, reply)
	} else if op.Type == LEAVE {
		args := LeaveArgs{op.GIDs, -1, -1}
		reply := sm.execute_leave(args)
		sm.updateCkRecords(op.Id, op.Ver, reply)
	} else if op.Type == MOVE {
		args := MoveArgs{op.Shard, op.GID, -1, -1}
		reply := sm.execute_move(args)
		sm.updateCkRecords(op.Id, op.Ver, reply)
	} else if op.Type == QUERY {
		args := QueryArgs{op.Num, -1, -1}
		reply := QueryReply{}
		nConfig := len(sm.configs)
		if args.Num >= nConfig || args.Num < 0 {
			reply.Config = sm.configs[nConfig-1]
		} else {
			reply.Config = sm.configs[args.Num]
		}
		sm.updateCkRecords(op.Id, op.Ver, reply)
	} else {
		panic("ShardMaster.execute: unknown op.Type")
	}
}

// when raft commit an message, handle it
func (sm *ShardMaster) msgHandler(applyCh chan raft.ApplyMsg) {
	for m := range applyCh {
		sm.mu.Lock()
		lastcommit := sm.lastcommit
		sm.mu.Unlock()
		idx := m.CommandIndex
		if m.CommandValid == false {
			// ignore other types of ApplyMsg
			sm.mu.Lock()
			sm.recover()
			sm.mu.Unlock()
		} else if idx <= lastcommit {
			// pass
			fmt.Printf(
				"SM Server %v: ignored ApplyMsg idx: %v, lastcommit: %v\n",
				sm.me, idx, lastcommit,
			)
			sm.snapshot(false)
		} else if idx > lastcommit + 1 {
			panic("sm.msgHandler: non-continuous commit")
		} else {
			op := m.Command.(Op)
			sm.cksMu.Lock()
			_, ok := sm.ckId2Idx[op.Id]
			if !ok {
				sm.createRecord(op.Id, 0)
			}
			sm.cksMu.Unlock()
			fmt.Printf(
				// "%v: committed %v at index %v\n",
				"\033[1;35m%v: committed %v at index %v\033[0m\n",
				sm.me, m.Command, m.CommandIndex,
			)
			sm.mu.Lock()
			sm.execute(op)
			sm.lastcommit++
			toSnapshot := sm.maxraftstate > 0 && sm.rf.StateSize() >= sm.maxraftstate
			sm.snapshot(toSnapshot)
			sm.mu.Unlock()
			sm.wakeup(idx)
		}
	}
}

// must be called with sm.mu held
func (sm *ShardMaster) snapshot(toSnapshot bool) {
	if !toSnapshot {
		sm.rf.DiscardBefore(-1, []byte{})
		return
	}
	// fmt.Printf("SM Server %v: Snapshotting...\n", sm.me)
	fmt.Printf("\033[1;32mSM Server %v: Snapshotting...\033[0m\n", sm.me)
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(sm.lastcommit)
	// TODO: encode []config
	nClerk := len(sm.cks)
	e.Encode(nClerk)
	for i := 0; i < nClerk; i++ {
		e.Encode(sm.cks[i].id)
		e.Encode(sm.cks[i].ver)
	}
	snapshot := w.Bytes()
	sm.rf.DiscardBefore(sm.lastcommit+1, snapshot)
}

// recover from snapshot
func (sm *ShardMaster) recover() {
	fmt.Printf("SM Server %v: Recover\n", sm.me)
	data := sm.persister.ReadSnapshot()
	if data == nil || len(data) < 1 {
		sm.lastcommit = 0
		return
	}
	d := labgob.NewDecoder(bytes.NewBuffer(data))
	var lastcommit int
	d.Decode(&lastcommit)
	sm.lastcommit = lastcommit
	// TODO: recover []config
	var nClerk int
	d.Decode(&nClerk)
	for i := 0; i < nClerk; i++ {
		var id, ver int64
		d.Decode(&id)
		d.Decode(&ver)
		sm.createRecord(id, ver)
	}
	return
}

// needed by shardsm tester
func (sm *ShardMaster) Raft() *raft.Raft {
	return sm.rf
}

//
// servers[] contains the ports of the set of
// servers that will cooperate via Paxos to
// form the fault-tolerant shardmaster service.
// me is the index of the current server in servers[].
//
func StartServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister) *ShardMaster {
	sm := new(ShardMaster)
	sm.me = me
	sm.maxraftstate = -1
	sm.persister = persister

	labgob.Register(Op{})
	// Your code here.
	fmt.Printf("\033[1;31mServer %v: initialized\033[0m\n", sm.me)
	// fmt.Printf("SM Server %v: initialized\n", sm.me)
	sm.conds = []*cond{newCond()}
	sm.configs = make([]Config, 1)
	sm.configs[0].Groups = map[int][]string{}
	sm.configVer = 0
	sm.ckId2Idx = make(map[int64]int)
	sm.applyCh = make(chan raft.ApplyMsg)
	sm.recover()
	go sm.msgHandler(sm.applyCh)

	sm.rf = raft.Make(servers, me, persister, sm.applyCh)

	return sm
}
