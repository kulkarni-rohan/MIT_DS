package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import "os"
import "strconv"

type EmptyArgs struct {}
type EmptyReply struct {}

type InitReply struct {
	Id int
	NReduce int
}

type AssignArgs struct {
	Id int
}

type AssignReply struct {
	File string
	Type int
}

type FinishArgs struct {
	Id int
	Type int
}

type NWorkerReply struct {
	NWorker int
}

type AliveArgs struct {
	Id int
}


// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the master.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func masterSock() string {
	s := "/var/tmp/824-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
