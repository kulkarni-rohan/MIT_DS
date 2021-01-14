package shardkv

//
// Sharded key/value server.
// Lots of replica groups, each running op-at-a-time paxos.
// Shardmaster decides which group serves each shard.
// Shardmaster may change shard assignment from time to time.
//
// You will have to modify these definitions.
//

const (
	OK             = "OK"
	ErrNoKey       = "ErrNoKey"
	ErrWrongGroup  = "ErrWrongGroup"
	ErrWrongLeader = "ErrWrongLeader"
	ErrNoContinuity = "ErrNoContinuity" // retry
	ErrUnknown 		= "ErrUnknown" // retry
)

type Err string

// Op.Op
const (
	GET		= "Get"
	PUT 	= "Put"
	APPEND  = "Append"
)

const (
	CONFIG404TIMEOUT = 200
	RPCTIMEOUT = 80
	WRONGGROUPWAIT = 100
	// check config every # milliseconds
	CONFIGINTERVAL = 100
)


// Put or Append
type PutAppendArgs struct {
	// You'll have to add definitions here.
	Key   string
	Value string
	Op    string // "Put" or "Append"
	// You'll have to add definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	Id    int64
	Ver	  int64
}

type PutAppendReply struct {
	Err Err
}

type GetArgs struct {
	Key string
	// You'll have to add definitions here. 4B
	Id    int64
	Ver	  int64
}

type GetReply struct {
	Err   Err
	Value string
}

// 4B
type GetShardArgs struct {
	Shard 	int
}

type GetShardReply struct {
	Err		Err
	Shard []byte
}

//-------------copied from https://golang.org/pkg/container/heap/ start--------
type IntHeap []int64
func (h IntHeap) Len() int           { return len(h) }
func (h IntHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h IntHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *IntHeap) Push(x interface{}) { *h = append(*h, x.(int64)) }
func (h *IntHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
//-------------copied from https://golang.org/pkg/container/heap/ end----------