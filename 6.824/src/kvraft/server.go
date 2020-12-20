package kvraft

import (
	"../labgob"
	"../labrpc"
	"../raft"
	"sync"
	"sync/atomic"
	"fmt"
)

const Debug = 1

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		fmt.Printf(format, a...)
	}
	return
}

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	Id	int64
	Ver int64
	Key string
	Val	string
	Op 	string
}

type cond struct {
	sync.Mutex
	cond *sync.Cond
}

type ck struct {
	idx int64
	ver int64
	records map[int64]interface{}
	// records []interface{}
	mu	sync.Mutex
}

// new a condition variable
func newCond() *cond {
	c := cond{}
	c.cond = sync.NewCond(&c)
	return &c
}

type KVServer struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg
	dead    int32 // set by Kill()

	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
	m 		map[string]string
	lastcommit	int // last commit index
	conds	[]*cond // condition variables
	cks		[]ck	// clerks
	ckId2Idx	map[int64]int // clerk Id to Index in clerks array
	cksMu	sync.Mutex // mutex for clerks
}

// extend condition variable to be at least as long as the client number
func (kv *KVServer) extendconds(idx int) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	for len(kv.conds) <= idx {
		kv.conds = append(kv.conds, newCond())
	}
}

// wait at idx
func (kv *KVServer) wait(idx int) {
	kv.extendconds(idx)
	kv.conds[idx].Lock()
	kv.conds[idx].cond.Wait()
	kv.conds[idx].Unlock()
}

// wakeup threads at idx
func (kv *KVServer) wakeup(idx int) {
	kv.extendconds(idx)
	kv.conds[idx].cond.Broadcast()
}

// update the ck committed records
func (kv *KVServer) updateCkRecords(id int64, ver int64, reply interface{}) {
	idx := kv.ckId2Idx[id]
	ck := &kv.cks[idx]
	fmt.Printf("Server %v: id: %v, ver: %v, ck.ver: %v\n", kv.me, ck.idx, ver, ck.ver)
	if ver != ck.ver + 1 {
		return
		// panic("kv.updateCkRecords: not continuous")
	}
	ck.ver++
	ck.records[ver] = reply
}

// must be called with kv.cksMu held
func (kv *KVServer) createRecord(id int64) {
	nCks := len(kv.cks)
	kv.cks = append(kv.cks, ck{id, 0, make(map[int64]interface{}), sync.Mutex{}})
	kv.ckId2Idx[id] = nCks
}

// if the ck does not exist, create it
// if exist, check if the records are continuous, if not, return an Error
// to let client wait
func (kv *KVServer) checkContinuity(id int64, ver int64) bool {
	kv.cksMu.Lock()
	defer kv.cksMu.Unlock()
	idx, ok := kv.ckId2Idx[id]
	if !ok {
		kv.createRecord(id)
		return true
	}
	ck := &kv.cks[idx]
	res := ver <= ck.ver + 1
	return res
}

// check check if the version of the opration is executed
// if exist, return it
func (kv *KVServer) checkSubmitted(id int64, ver int64) (interface{}, bool) {
	kv.cksMu.Lock()
	defer kv.cksMu.Unlock()
	idx := kv.ckId2Idx[id]
	ck := &kv.cks[idx]
	if ck.ver >= ver {
		fmt.Printf("%v: Loading\n", kv.me)
		return ck.records[ver], true
	}
	return 1, false
}


func (kv *KVServer) submit(cmd Op) int {
	lastcommit := kv.lastcommit
	idx, _, isLeader := kv.rf.Start(cmd)
	if !isLeader {
		return 0
	}
	if idx <= lastcommit {
		return 0
		// panic("kv.submit: outdated commitindex")
	} 
	return idx
}

// Same for Get and PutAppend
// 1. check if the request numbers are continuous, if not, let client wait
// 2. check if duplicate, if yes, return directly
// 3. check if I am the leader, if not, return
// 4. submit the operation
// 5. read operation from record
func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	// Your code here.
	if !kv.checkContinuity(args.Id, args.Ver) {
		reply.Err = ErrNoContinuity
		return
	}
	oldReply, isSubmitted := kv.checkSubmitted(args.Id, args.Ver)
	if isSubmitted {
		reply.Err = oldReply.(GetReply).Err
		reply.Value = oldReply.(GetReply).Value
		fmt.Printf("Server %v: return submitted: %v %v\n", kv.me, args, reply)
		return
	}
	cmd := Op{args.Id, args.Ver, args.Key, "", "Get"}
	idx := kv.submit(cmd)
	if idx == 0 {
		reply.Err = ErrWrongLeader
		return
	}
	kv.wait(idx)
	oldReply, isSubmitted = kv.checkSubmitted(args.Id, args.Ver)
	if isSubmitted {
		reply.Err = oldReply.(GetReply).Err
		reply.Value = oldReply.(GetReply).Value
		return
	} else {
		// panic("kv.Get: submitted but cannot find")
		reply.Err = ErrWrongLeader
		return
	}
}

// Same as Get
func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	if !kv.checkContinuity(args.Id, args.Ver) {
		reply.Err = ErrNoContinuity
		return
	}
	_, isSubmitted := kv.checkSubmitted(args.Id, args.Ver)
	if isSubmitted {
		return
	}
	cmd := Op{args.Id, args.Ver, args.Key, args.Value, args.Op}
	idx := kv.submit(cmd)
	if idx == 0 {
		reply.Err = ErrWrongLeader
		return
	}
	kv.wait(idx)
}

//
// the tester calls Kill() when a KVServer instance won't
// be needed again. for your convenience, we supply
// code to set rf.dead (without needing a lock),
// and a killed() method to test rf.dead in
// long-running loops. you can also add your own
// code to Kill(). you're not required to do anything
// about this, but it may be convenient (for example)
// to suppress debug output from a Kill()ed instance.
//
func (kv *KVServer) Kill() {
	atomic.StoreInt32(&kv.dead, 1)
	kv.rf.Kill()
	// Your code here, if desired.
}

func (kv *KVServer) killed() bool {
	z := atomic.LoadInt32(&kv.dead)
	return z == 1
}

// execute operation
func (kv *KVServer) execute(op Op) {
	idx := kv.ckId2Idx[op.Id]
	ck := &kv.cks[idx]
	ck.mu.Lock()
	defer ck.mu.Unlock()
	_, ok := kv.checkSubmitted(op.Id, op.Ver)
	if ok {
		return
	}
	if op.Op == PUT {
		kv.m[op.Key] = op.Val
		reply := PutAppendReply{}
		kv.updateCkRecords(op.Id, op.Ver, reply)
	} else if op.Op == APPEND {
		kv.m[op.Key] += op.Val
		reply := PutAppendReply{}
		kv.updateCkRecords(op.Id, op.Ver, reply)
	} else { // GET
		reply := GetReply{}
		v, ok := kv.m[op.Key]
		if !ok {
			reply.Err = ErrNoKey
		} else {
			reply.Value = v
		}
		kv.updateCkRecords(op.Id, op.Ver, reply)
	}
}

// when raft commit an message, handle it
func (kv *KVServer) msgHandler(applyCh chan raft.ApplyMsg) {
	for m := range applyCh {
		idx := m.CommandIndex
		if m.CommandValid == false {
			// ignore other types of ApplyMsg
		} else if idx <= kv.lastcommit {
			// pass
		} else if idx > kv.lastcommit + 1 {
			panic("kv.msgHandler: non-continuous commit")
		} else {
			op := m.Command.(Op)
			kv.cksMu.Lock()
			_, ok := kv.ckId2Idx[op.Id]
			if !ok {
				kv.createRecord(op.Id)
			}
			kv.cksMu.Unlock()
			fmt.Printf(
				"%v: committed %v at index %v\n",
				// "\033[1;35m%v: committed %v at index %v\033[0m\n",
				kv.me, m.Command, m.CommandIndex,
			)
			kv.execute(op)
			kv.lastcommit++
			kv.wakeup(idx)
		}
	}
}

func (kv *KVServer) recover() {
// 	kv.rf.mu.Lock()
// 	defer kv.rf.mu.Unlock()
}

//
// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
// me is the index of the current server in servers[].
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
// the k/v server should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
// StartKVServer() must return quickly, so it should start goroutines
// for any long-running work.
//
func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *KVServer {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(Op{})

	kv := new(KVServer)
	kv.me = me
	kv.maxraftstate = maxraftstate

	// You may need initialization code here.

	// fmt.Printf("\033[1;31mServer %v: initialized\033[0m\n", kv.me)
	fmt.Printf("Server %v: initialized\n", kv.me)
	kv.conds = []*cond{newCond()}
	kv.m = make(map[string]string)
	kv.ckId2Idx = make(map[int64]int)
	kv.applyCh = make(chan raft.ApplyMsg)
	go kv.msgHandler(kv.applyCh)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)
	kv.recover()

	// You may need initialization code here.

	return kv
}
