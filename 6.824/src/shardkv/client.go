package shardkv

//
// client code to talk to a sharded key/value service.
//
// the client first talks to the shardmaster to find out
// the assignment of shards (keys) to groups, and then
// talks to the group that holds the key's shard.
//

import "../labrpc"
import "crypto/rand"
import "math/big"
import "../shardmaster"
import "time"
import "sync"
import "fmt"
import "container/heap"

//
// which shard is a key in?
// please use this function,
// and please do not change it.
//
func key2shard(key string) int {
	shard := 0
	if len(key) > 0 {
		shard = int(key[0])
	}
	shard %= shardmaster.NShards
	return shard
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

type Clerk struct {
	sm       *shardmaster.Clerk
	config   shardmaster.Config
	make_end func(string) *labrpc.ClientEnd
	// You will have to modify this struct.
	mu		sync.Mutex
	id		int64
	ver		map[int]*IntHeap
	// ver		[shardmaster.NShards]int64
	isLeader	[]bool
}

//
// the tester calls MakeClerk.
//
// masters[] is needed to call shardmaster.MakeClerk().
//
// make_end(servername) turns a server name from a
// Config.Groups[gid][i] into a labrpc.ClientEnd on which you can
// send RPCs.
//
func MakeClerk(masters []*labrpc.ClientEnd, make_end func(string) *labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.sm = shardmaster.MakeClerk(masters)
	ck.make_end = make_end
	// You'll have to add code here.
	ck.id = nrand()
	ck.ver = make(map[int]*IntHeap)
	fmt.Printf("Clerk %v: initialized\n", ck.id)
	return ck
}

// assign a new version number
// if have previously failed version, use it, otherwise use new version
func (ck *Clerk) newVer(shard int) int64 {
	ck.mu.Lock()
	defer ck.mu.Unlock()
	if ck.ver[shard] == nil {
		ck.ver[shard] = &IntHeap{0}
	}
	res := heap.Pop(ck.ver[shard]).(int64) + 1
	if ck.ver[shard].Len() == 0 {
		heap.Push(ck.ver[shard], res)
	}
	// for i := 0; i < shardmaster.NShards; i++ {
	// 	fmt.Printf("%v %v\n", i, ck.ver[i])
	// }
	return res
}

// put the failed version back in heap
func (ck *Clerk) releaseVer(shard int, ver int64) {
	ck.mu.Lock()
	defer ck.mu.Unlock()
	fmt.Printf("--------------------------------------\n")
	heap.Push(ck.ver[shard], ver-1)
}

//
// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
// You will have to modify this function.
//
func (ck *Clerk) Get(key string) string {
	shard := key2shard(key)
	for {
		gid := ck.config.Shards[shard]
		ver := ck.newVer(shard)
		args := GetArgs{key, ck.id, ver}
		servers, ok := []string{}, false
		// if not found in existing config
		if servers, ok = ck.config.Groups[gid]; !ok {
			time.Sleep(CONFIG404TIMEOUT * time.Millisecond)
			ck.config = ck.sm.Query(-1)
			ck.releaseVer(shard, ver)
			continue
		}
		for {
			ok := false
			wrongGroup := false
			res := ""
			for i, _ := range servers {
				go func(i int) {
					server := ck.make_end(servers[i])
					reply := GetReply{}
					okk := server.Call("ShardKV.Get", &args, &reply)
					if okk {
						fmt.Printf("get -> %v-%v args: %v, reply: %v, shard: %v\n", gid, i, args, reply, shard)
					} else {
						fmt.Printf("get -> %v-%v network failure\n", gid, i)
					}
					ck.mu.Lock()
					defer ck.mu.Unlock()
					if okk && reply.Err == "" {
						res = reply.Value
						ok = true
					} else if reply.Err == ErrNoKey {
						res = ""
						ok = true
					} else if reply.Err == ErrWrongLeader {
						// ck.isLeader[i] = false
					} else if reply.Err == ErrWrongGroup {
						wrongGroup = true
					}
				}(i)
			}
			time.Sleep(RPCTIMEOUT* time.Millisecond)
			ck.mu.Lock()
			if ok {
				ck.mu.Unlock()
				return res
			} else if wrongGroup {
				time.Sleep(WRONGGROUPWAIT * time.Millisecond)
				ck.config = ck.sm.Query(-1)
				ck.mu.Unlock()
				break
			}
			ck.mu.Unlock()
		}
		ck.releaseVer(shard, ver)
	}
}

//
// shared by Put and Append.
// You will have to modify this function.
//
func (ck *Clerk) PutAppend(key string, value string, op string) {
	shard := key2shard(key)
	for {
		gid := ck.config.Shards[shard]
		ver := ck.newVer(shard)
		args := PutAppendArgs{key, value, op, ck.id, ver}
		servers, ok := []string{}, false
		// if not found in existing config
		if servers, ok = ck.config.Groups[gid]; !ok {
			time.Sleep(CONFIG404TIMEOUT * time.Millisecond)
			ck.config = ck.sm.Query(-1)
			ck.releaseVer(shard, ver)
			continue
		}
		for {
			ok := false
			wrongGroup := false
			for i, _ := range servers {
				go func(i int) {
					server := ck.make_end(servers[i])
					reply := GetReply{}
					okk := server.Call("ShardKV.PutAppend", &args, &reply)
					if okk {
						fmt.Printf("PA -> %v-%v args: %v, reply: %v\n", gid, i, args, reply)
					} else {
						fmt.Printf("PA -> %v-%v network failure\n", gid, i)
					}
					ck.mu.Lock()
					defer ck.mu.Unlock()
					if okk && reply.Err == "" {
						ok = true
					} else if reply.Err == ErrWrongLeader {
						// ck.isLeader[i] = false
					} else if reply.Err == ErrWrongGroup {
						wrongGroup = true
					}
				}(i)
			}
			time.Sleep(RPCTIMEOUT* time.Millisecond)
			ck.mu.Lock()
			if ok {
				ck.mu.Unlock()
				return
			} else if wrongGroup {
				time.Sleep(WRONGGROUPWAIT * time.Millisecond)
				ck.config = ck.sm.Query(-1)
				ck.mu.Unlock()
				break
			}
			ck.mu.Unlock()
		}
		ck.releaseVer(shard, ver)
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}
