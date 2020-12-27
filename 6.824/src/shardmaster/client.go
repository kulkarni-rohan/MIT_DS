package shardmaster

//
// Shardmaster clerk.
//

import "../labrpc"
import "time"
import "crypto/rand"
import "math/big"
import (
	"sync"
	"fmt"
)

type Clerk struct {
	servers []*labrpc.ClientEnd
	// Your data here.
	mu		sync.Mutex
	id		int64
	ver		int64
	isLeader	[]bool
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	// Your code here.
	ck.id = nrand()
	ck.ver = 0
	ck.isLeader = make([]bool, len(ck.servers))
	// fmt.Printf("SM Clerk %v: initialized\n", ck.id)
	fmt.Printf("\033[1;31mClerk %v: initialized\033[0m\n", ck.id)
	return ck
}

// set all entries of isLeader to true
func (ck *Clerk) restore() {
	for i := 0; i < len(ck.isLeader); i++ {
		ck.isLeader[i] = true
	}
}

// first call returns 1, assign a version number to each
// Get / PutAppend request
func (ck *Clerk) newVer() int64 {
	ck.mu.Lock()
	defer ck.mu.Unlock()
	ck.ver++
	res := ck.ver
	return res
}

func (ck *Clerk) Query(num int) Config {
	// Your code here.
	args := QueryArgs{num, ck.id, ck.newVer()}
	for {
		ok := false
		res := Config{}
		for i, _ := range ck.servers {
			if !ck.isLeader[i] {
				continue
			}
			go func(i int) {
				srv := ck.servers[i]
				reply := QueryReply{}
				okk := srv.Call("ShardMaster.Query", &args, &reply)
				if okk {
					fmt.Printf("Query -> %v args: %v, reply: %v\n", i, args, reply)
				} else {
					fmt.Printf("Query -> %v network failure\n", i)
				}
				ck.mu.Lock()
				defer ck.mu.Unlock()
				if okk && reply.Err == "" && !reply.WrongLeader {
					res = reply.Config
					ok = true
				} else if reply.WrongLeader {
					ck.isLeader[i] = false
				}
			}(i)
		}
		time.Sleep(RPCTIMEOUT * time.Millisecond)
		ck.mu.Lock()
		if ok {
			ck.mu.Unlock()
			return res
		}
		ck.restore()
		ck.mu.Unlock()
	}
}

func (ck *Clerk) Join(servers map[int][]string) {
	// Your code here.
	args := JoinArgs{servers, ck.id, ck.newVer()}
	for {
		ok := false
		for i, _ := range ck.servers {
			if !ck.isLeader[i] {
				continue
			}
			go func(i int) {
				srv := ck.servers[i]
				reply := JoinReply{}
				okk := srv.Call("ShardMaster.Join", &args, &reply)
				ck.mu.Lock()
				defer ck.mu.Unlock()
				if okk && reply.Err == "" && !reply.WrongLeader {
					ok = true
				} else if reply.WrongLeader {
					ck.isLeader[i] = false
				}
			}(i)
		}
		time.Sleep(RPCTIMEOUT * time.Millisecond)
		ck.mu.Lock()
		if ok {
			ck.mu.Unlock()
			return
		}
		ck.restore()
		ck.mu.Unlock()
	}
}

func (ck *Clerk) Leave(gids []int) {
	// Your code here.
	args := LeaveArgs{gids, ck.id, ck.newVer()}
	for {
		ok := false
		for i, _ := range ck.servers {
			if !ck.isLeader[i] {
				continue
			}
			go func(i int) {
				srv := ck.servers[i]
				reply := LeaveReply{}
				okk := srv.Call("ShardMaster.Leave", &args, &reply)
				ck.mu.Lock()
				defer ck.mu.Unlock()
				if okk && reply.Err == "" && !reply.WrongLeader {
					ok = true
				} else if reply.WrongLeader {
					ck.isLeader[i] = false
				}
			}(i)
		}
		time.Sleep(RPCTIMEOUT * time.Millisecond)
		ck.mu.Lock()
		if ok {
			ck.mu.Unlock()
			return
		}
		ck.restore()
		ck.mu.Unlock()
	}
}

func (ck *Clerk) Move(shard int, gid int) {
	// Your code here.
	args := MoveArgs{shard, gid, ck.id, ck.newVer()}
	for {
		ok := false
		for i, _ := range ck.servers {
			if !ck.isLeader[i] {
				continue
			}
			go func(i int) {
				srv := ck.servers[i]
				reply := MoveReply{}
				okk := srv.Call("ShardMaster.Move", &args, &reply)
				ck.mu.Lock()
				defer ck.mu.Unlock()
				if okk && reply.Err == "" && !reply.WrongLeader {
					ok = true
				} else if reply.WrongLeader {
					ck.isLeader[i] = false
				}
			}(i)
		}
		time.Sleep(RPCTIMEOUT * time.Millisecond)
		ck.mu.Lock()
		if ok {
			ck.mu.Unlock()
			return
		}
		ck.restore()
		ck.mu.Unlock()
	}
}
