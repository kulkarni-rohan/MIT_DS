package kvraft

import "../labrpc"
import "crypto/rand"
import "math/big"
import "sync"
import "time"
import "fmt"


type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
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

// use a 64-bit random number as cliend id
func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	// You'll have to add code here.
	ck.id = nrand()
	ck.ver = 0
	ck.isLeader = make([]bool, len(ck.servers))
	fmt.Printf("Clerk %v: initialized\n", ck.id)
	// fmt.Printf("\033[1;31mClerk %v: initialized\033[0m\n", ck.id)
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

//
// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) Get(key string) string {

	// You will have to modify this function.
	args := GetArgs{key, ck.id, ck.newVer()}
	for {
		ok := false
		res := ""
		for i, _ := range ck.servers {
			if !ck.isLeader[i] {
				continue
			}
			go func(i int) {
				server := ck.servers[i]
				reply := GetReply{}
				okk := server.Call("KVServer.Get", &args, &reply)
				if okk {
					fmt.Printf("get -> %v args: %v, reply: %v\n", i, args, reply)
				} else {
					fmt.Printf("get -> %v network failure\n", i)
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
					ck.isLeader[i] = false
				}
			}(i)
		}
		time.Sleep(RPCTIMEOUT* time.Millisecond)
		ck.mu.Lock()
		if ok {
			ck.mu.Unlock()
			return res
		}
		ck.restore()
		ck.mu.Unlock()
	}
}

//
// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.PutAppend", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) PutAppend(key string, value string, op string) {
	// You will have to modify this function.
	args := PutAppendArgs{key, value, op, ck.id, ck.newVer()}
	for {
		ok := false
		for i, _ := range ck.servers {
			if !ck.isLeader[i] {
				continue
			}
			go func(i int) {
				server := ck.servers[i]
				reply := PutAppendReply{}
				okk := server.Call("KVServer.PutAppend", &args, &reply)
				ck.mu.Lock()
				defer ck.mu.Unlock()
				if okk && reply.Err == "" {
					ok = true
				} else if reply.Err == ErrWrongLeader {
					ck.isLeader[i] = false
				}
			}(i)
		}
		time.Sleep(RPCTIMEOUT* time.Millisecond)
		ck.mu.Lock()
		if ok {
			ck.mu.Unlock()
			return
		}
		ck.restore()
		ck.mu.Unlock()
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}
