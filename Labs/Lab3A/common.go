package kvraft

const (
	OK             = "OK"
	ErrNoKey       = "ErrNoKey" // no need retry
	ErrWrongLeader = "ErrWrongLeader" // retry
	ErrNoContinuity = "ErrNoContinuity" // retry
)

// Op.Op
const (
	GET		= "Get"
	PUT 	= "Put"
	APPEND  = "Append"
)

// interval between each round of Get/PutAppend in ms
const (
	RPCTIMEOUT = 200
)

type Err string

// Put or Append
type PutAppendArgs struct {
	Key   string
	Value string
	Op    string // "Put" or "Append"
	// You'll have to add definitions here.
	Id    int64
	Ver	  int64
}

type PutAppendReply struct {
	Err Err
}

type GetArgs struct {
	Key string
	// You'll have to add definitions here.
	Id    int64
	Ver	  int64
}

type GetReply struct {
	Err   Err
	Value string
}
