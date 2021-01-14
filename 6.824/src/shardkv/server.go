
package shardkv


import "../shardmaster"
import "../labrpc"
import "../raft"
import "sync"
import "../labgob"
import "fmt"
import "bytes"
import "time"


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
	id int64
	ver [shardmaster.NShards]int64
	// ver int64
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

// return idx of b in A, if not found, return -1
func find(A []int, b int) int {
	for i, v := range A {
		if v == b {
			return i
		}
	}
	return -1
}

// remove index i from A
func remove(A []int, i int) {
	if i == len(A) - 1 {
		A = A[:i]
	} else {
		A = append(A[:i], A[i+1:]...)
	}
}

type ShardKV struct {
	mu           sync.Mutex
	me           int
	rf           *raft.Raft
	applyCh      chan raft.ApplyMsg
	make_end     func(string) *labrpc.ClientEnd
	gid          int
	masters      []*labrpc.ClientEnd
	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
	m 		map[string]string
	lastcommit	int // last commit index
	conds	[]*cond // condition variables
	cks		[]ck	// clerks
	ckId2Idx	map[int64]int // clerk Id to Index in clerks array
	cksMu	sync.Mutex // mutex for clerks
	persister	*raft.Persister
	mck		*shardmaster.Clerk
	config	*shardmaster.Config
	lastConfigTime int64
	inCharge []int // shards in charge
}

// extend condition variable to be at least as long as the client number
func (kv *ShardKV) extendconds(idx int) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	for len(kv.conds) <= idx {
		kv.conds = append(kv.conds, newCond())
	}
}

// wait at idx
func (kv *ShardKV) wait(idx int) {
	kv.extendconds(idx)
	kv.conds[idx].Lock()
	kv.conds[idx].cond.Wait()
	kv.conds[idx].Unlock()
}

// wakeup threads at idx
func (kv *ShardKV) wakeup(idx int) {
	kv.extendconds(idx)
	kv.conds[idx].cond.Broadcast()
}

// update the ck committed records
func (kv *ShardKV) updateCkRecords(id int64, ver int64, reply interface{}, shard int) {
	idx := kv.ckId2Idx[id]
	ck := &kv.cks[idx]
	fmt.Printf("KV server-%v-%v: id: %v, shard: %v, ver: %v, ck.ver: %v\n", kv.gid, kv.me, ck.id, shard, ver, ck.ver[shard])
	if ver != ck.ver[shard] + 1 {
		return
		// panic("kv.updateCkRecords: not continuous")
	}
	ck.ver[shard]++
	ck.records[ver] = reply
}

// must be called with kv.cksMu held
func (kv *ShardKV) createRecord(id int64, ver [shardmaster.NShards]int64) {
	nCks := len(kv.cks)
	kv.cks = append(kv.cks, ck{id, ver, make(map[int64]interface{}), sync.Mutex{}})
	kv.ckId2Idx[id] = nCks
}

// if the ck does not exist, create it
// if exist, check if the records are continuous, if not, return an Error
// to let client wait
func (kv *ShardKV) checkContinuity(id int64, ver int64, shard int) bool {
	kv.cksMu.Lock()
	defer kv.cksMu.Unlock()
	idx, ok := kv.ckId2Idx[id]
	if !ok {
		kv.createRecord(id, [shardmaster.NShards]int64{})
	}
	ck := &kv.cks[idx]
	res := ver <= ck.ver[shard] + 1
	return res
}

// check check if the version of the opration is executed
// if exist, return it
func (kv *ShardKV) checkSubmitted(id int64, ver int64, shard int) (interface{}, bool) {
	kv.cksMu.Lock()
	defer kv.cksMu.Unlock()
	idx := kv.ckId2Idx[id]
	ck := &kv.cks[idx]
	record := ck.records[ver]
	if ck.ver[shard] >= ver {
		fmt.Printf("%v-%v: Loading, ver: %v, ck.ver: %v\n", kv.gid, kv.me, ver, ck.ver)
		return record, true
	}
	return 1, false
}

func (kv *ShardKV) submit(cmd Op) int {
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

func (kv *ShardKV) decodeShard(shard []byte) {
	d := labgob.NewDecoder(bytes.NewBuffer(shard))
	for i := 0; i < shardmaster.NShards; i++ {
		var id, ver int64
		d.Decode(&id)
		d.Decode(&ver)
		idx := kv.ckId2Idx[id]
		kv.cks[idx].ver[i] = ver
	}
	var cnt int
	d.Decode(&cnt)
	for i := 0; i < cnt; i++ {
		var k, v string
		d.Decode(&k)
		d.Decode(&v)
		kv.m[k] = v
	}
}

// caller must hold kv.mu
func (kv *ShardKV) checkConfig() *shardmaster.Config {
	// fmt.Printf("time.Now().Unix():%v, kv.lastConfigTime: %v\n", time.Now().UnixNano(), kv.lastConfigTime)
	if time.Now().UnixNano() - kv.lastConfigTime > CONFIGINTERVAL * 1e6 {
		oldConfig := kv.config
		newConfig := kv.mck.Query(-1)
		kv.config = &newConfig
		kv.lastConfigTime = time.Now().UnixNano()
		// fmt.Printf("--------------------%v\n", kv.config)
		for i := shardmaster.NShards - 1; i >= 0; i-- {
			idx := find(kv.inCharge,i)
			if kv.config.Shards[i] == kv.gid && idx == -1 {
				// new assigned
				kv.inCharge = append(kv.inCharge, i)
				if _, isLeader := kv.rf.GetState(); !isLeader {
					break
				}
				args := GetShardArgs{i} 
				shard := []byte{}
				for {
					oldgid := oldConfig.Shards[i]
					servers := oldConfig.Groups[oldgid]
					ok := false
					mu := sync.Mutex{}
					if len(servers) == 0 {
						break
					}
					for _, serverName := range servers {
						go func(serverName *string) {
							srv := kv.make_end(*serverName)
							reply := GetShardReply{}
							okk := srv.Call("ShardKV.GetShard", &args, &reply)
							if okk && reply.Err == "" {
								mu.Lock()
								ok = true
								mu.Unlock()
								shard = reply.Shard
								fmt.Printf("success")
							} else if okk {
								fmt.Printf("%v-%v -> %v: %v\n", kv.gid, kv.me, *serverName, reply.Err)
							}
						} (&serverName)
					}
					time.Sleep((RPCTIMEOUT-20)*time.Millisecond)
					mu.Lock()
					if ok {
						kv.decodeShard(shard)
						mu.Unlock()
						break
					}
					mu.Unlock()
				}
			} else if kv.config.Shards[i] != kv.gid && idx != -1 {
				// deprecated
				remove(kv.inCharge, idx)
			}
		}
	}
	return kv.config
}

func (kv *ShardKV) checkGroup(key string) bool {
	// server-[gid]-[kv.me]
	kv.mu.Lock()
	defer kv.mu.Unlock()
	config := kv.checkConfig()
	shard := key2shard(key)
	return config.Shards[shard] == kv.gid
}

// Same for Get and PutAppend
// 1. check if the request numbers are continuous, if not, let client wait
// 2. check if duplicate, if yes, return directly
// 3. check if I am the leader, if not, return
// 4. submit the operation
// 5. read operation from record
func (kv *ShardKV) Get(args *GetArgs, reply *GetReply) {
	// Your code here.
	shard := key2shard(args.Key)
	if !kv.checkGroup(args.Key) {
		reply.Err = ErrWrongGroup
		return
	}
	if !kv.checkContinuity(args.Id, args.Ver, shard) {
		reply.Err = ErrNoContinuity
		return
	}
	oldReply, isSubmitted := kv.checkSubmitted(args.Id, args.Ver, shard)
	if oldReply == nil {
		v, ok := kv.m[args.Key]
		if !ok {
			reply.Err = ErrNoKey
		} else {
			reply.Value = v
		}
		return
	}
	if isSubmitted {
		reply.Err = oldReply.(GetReply).Err
		reply.Value = oldReply.(GetReply).Value
		fmt.Printf("KV server-%v-%v: return submitted: %v %v\n", kv.gid, kv.me, args, reply)
		return
	}
	cmd := Op{args.Id, args.Ver, args.Key, "", "Get"}
	idx := kv.submit(cmd)
	if idx == 0 {
		reply.Err = ErrWrongLeader
		return
	}
	kv.wait(idx)
	oldReply, isSubmitted = kv.checkSubmitted(args.Id, args.Ver, shard)
	if oldReply == nil {
		v, ok := kv.m[args.Key]
		if !ok {
			reply.Err = ErrNoKey
		} else {
			reply.Value = v
		}
		return
	}
	if isSubmitted {
		reply.Err = oldReply.(GetReply).Err
		reply.Value = oldReply.(GetReply).Value
		return
	} else {
		// panic("kv.Get: submitted but cannot find")
		reply.Err = ErrUnknown
		return
	}
}

func (kv *ShardKV) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	shard := key2shard(args.Key)
	if !kv.checkGroup(args.Key) {
		reply.Err = ErrWrongGroup
		return
	}
	if !kv.checkContinuity(args.Id, args.Ver, shard) {
		reply.Err = ErrNoContinuity
		return
	}
	_, isSubmitted := kv.checkSubmitted(args.Id, args.Ver, shard)
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

func (kv *ShardKV) GetShard(args *GetShardArgs, reply *GetShardReply) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	fmt.Printf("%v-%v: GetShard %v\n", kv.gid, kv.me, args.Shard)
	_, isLeader := kv.rf.GetState()
	if !isLeader {
		reply.Err = ErrWrongLeader
	}
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	for i := 0; i < len(kv.cks); i++ {
		ck := &kv.cks[i]
		e.Encode(ck.id)
		e.Encode(ck.ver[args.Shard])
	}
	cnt := 0
	tobeEncode := []*string{}
	for k, v := range kv.m {
		if key2shard(k) == args.Shard {
			tobeEncode = append(tobeEncode, &k, &v)
			cnt++
		}
	}
	e.Encode(cnt)
	for _, v := range tobeEncode {
		e.Encode(*v)
	}
	reply.Shard = w.Bytes()
	return
}

//
// the tester calls Kill() when a ShardKV instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (kv *ShardKV) Kill() {
	kv.rf.Kill()
	// Your code here, if desired.
}

// execute operation
func (kv *ShardKV) execute(op Op) {
	idx := kv.ckId2Idx[op.Id]
	ck := &kv.cks[idx]
	ck.mu.Lock()
	defer ck.mu.Unlock()
	shard := key2shard(op.Key)
	_, ok := kv.checkSubmitted(op.Id, op.Ver, shard)
	if ok {
		return
	}
	if op.Op == PUT {
		kv.m[op.Key] = op.Val
		reply := PutAppendReply{}
		kv.updateCkRecords(op.Id, op.Ver, reply, shard)
	} else if op.Op == APPEND {
		kv.m[op.Key] += op.Val
		reply := PutAppendReply{}
		kv.updateCkRecords(op.Id, op.Ver, reply, shard)
	} else { // GET
		reply := GetReply{}
		v, ok := kv.m[op.Key]
		if !ok {
			reply.Err = ErrNoKey
		} else {
			reply.Value = v
		}
		kv.updateCkRecords(op.Id, op.Ver, reply, shard)
	}
}

// when raft commit an message, handle it
func (kv *ShardKV) msgHandler(applyCh chan raft.ApplyMsg) {
	for m := range applyCh {
		idx := m.CommandIndex
		if m.CommandValid == false {
			// ignore other types of ApplyMsg
			kv.mu.Lock()
			kv.recover()
			kv.mu.Unlock()
		} else if idx <= kv.lastcommit {
			// pass
			fmt.Printf(
				"KV server-%v-%v: ignored ApplyMsg idx: %v, lastcommit: %v\n", 
				kv.gid, kv.me, idx, kv.lastcommit,
			)
			kv.snapshot(false, false)
		} else if idx > kv.lastcommit + 1 {
			panic("kv.msgHandler: non-continuous commit")
		} else {
			op := m.Command.(Op)
			kv.cksMu.Lock()
			_, ok := kv.ckId2Idx[op.Id]
			if !ok {
				kv.createRecord(op.Id, [shardmaster.NShards]int64{})
			}
			kv.cksMu.Unlock()
			fmt.Printf(
				// "%v-%v: committed %v at index %v\n",
				"\033[1;35m%v-%v: committed %v at index %v\033[0m\n",
				kv.gid, kv.me, m.Command, m.CommandIndex,
			)
			kv.mu.Lock()
			kv.execute(op)
			kv.lastcommit++
			toSnapshot := kv.maxraftstate > 0 && kv.rf.StateSize() >= kv.maxraftstate
			kv.
			snapshot(toSnapshot, true)
			kv.mu.Unlock()
			kv.wakeup(idx)
		}
	}
}

// must be called with kv.mu held
func (kv *ShardKV) snapshot(toSnapshot bool, waited bool) {
	if !toSnapshot {
		kv.rf.DiscardBefore(-1, []byte{}, true)
		return
	}
	fmt.Printf("KV server-%v-%v: Snapshotting...\n", kv.gid, kv.me)
	// fmt.Printf("\033[1;32mKV server-%v-%v: Snapshotting...\033[0m\n", kv.me)
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(kv.lastcommit)
	e.Encode(len(kv.m))
	for k, v := range kv.m {
		e.Encode(k)
		e.Encode(v)
	}
	nClerk := len(kv.cks)
	e.Encode(nClerk)
	for i := 0; i < nClerk; i++ {
		e.Encode(kv.cks[i].id)
		for j := 0; j < shardmaster.NShards; j++ {
			e.Encode(kv.cks[i].ver[j])
		}
	}
	snapshot := w.Bytes()
	kv.rf.DiscardBefore(kv.lastcommit+1, snapshot, waited)
}

// recover from snapshot
func (kv *ShardKV) recover() {
	fmt.Printf("KV server-%v-%v: Recover\n", kv.gid, kv.me)
	data := kv.persister.ReadSnapshot()
	if data == nil || len(data) < 1 {
		kv.lastcommit = 0
		return
	}
	d := labgob.NewDecoder(bytes.NewBuffer(data))
	var nKey, lastcommit int
	d.Decode(&lastcommit)
	d.Decode(&nKey)
	kv.lastcommit = lastcommit
	for i := 0; i < nKey; i++ {
		var k, v string
		d.Decode(&k)
		d.Decode(&v)
		kv.m[k] = v
	}
	var nClerk int
	d.Decode(&nClerk)
	for i := 0; i < nClerk; i++ {
		var id int64
		var ver [shardmaster.NShards]int64
		d.Decode(&id)
		for j := 0; j < shardmaster.NShards; j++ {
			d.Decode(&ver[j])
		}
		kv.createRecord(id, ver)
	}
	return
}

func (kv *ShardKV) configChecker() {
	for {
		kv.mu.Lock()
		kv.checkConfig()
		kv.mu.Unlock()
		time.Sleep(CONFIGINTERVAL * time.Millisecond)
	}
}

//
// servers[] contains the ports of the servers in this group.
//
// me is the index of the current server in servers[].
//
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
//
// the k/v server should snapshot when Raft's saved state exceeds
// maxraftstate bytes, in order to allow Raft to garbage-collect its
// log. if maxraftstate is -1, you don't need to snapshot.
//
// gid is this group's GID, for interacting with the shardmaster.
//
// pass masters[] to shardmaster.MakeClerk() so you can send
// RPCs to the shardmaster.
//
// make_end(servername) turns a server name from a
// Config.Groups[gid][i] into a labrpc.ClientEnd on which you can
// send RPCs. You'll need this to send RPCs to other groups.
//
// look at client.go for examples of how to use masters[]
// and make_end() to send RPCs to the group owning a specific shard.
//
// StartServer() must return quickly, so it should start goroutines
// for any long-running work.
//
func StartServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, 
	maxraftstate int, gid int, masters []*labrpc.ClientEnd, make_end func(string) *labrpc.ClientEnd) *ShardKV {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(Op{})

	kv := new(ShardKV)
	kv.me = me
	kv.maxraftstate = maxraftstate
	kv.make_end = make_end
	kv.gid = gid
	kv.masters = masters
	kv.persister = persister

	// Your initialization code here.
	// Use something like this to talk to the shardmaster:
	// kv.mck = shardmaster.MakeClerk(kv.masters)
	fmt.Printf("\033[1;31mKV server-%v-%v: initialized\033[0m\n", kv.gid, kv.me)
	// fmt.Printf("KV KV server-%v-%v: initialized with gid %v\n", kv.me, kv.gid)

	kv.conds = []*cond{newCond()}
	kv.m = make(map[string]string)
	kv.ckId2Idx = make(map[int64]int)
	kv.applyCh = make(chan raft.ApplyMsg)
	nMaster := len(masters)
	masters = append(masters, masters[nMaster-1])
	kv.mck = shardmaster.MakeClerk(masters)
	kv.recover()
	go kv.msgHandler(kv.applyCh)
	go kv.configChecker()
	kv.rf = raft.Make(servers, (gid<<5)|me, persister, kv.applyCh)
	return kv
}
