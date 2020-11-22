package mr

import "log"
import "net"
import "os"
import "net/rpc"
import "net/http"
import "sync"
import "fmt"
import "time"

type Task struct {
	lock sync.Mutex
	status int
	worker int
	file string
	tipe int
}

type WorkerStat struct {
	lastseen int64
	alive bool
	lock sync.Mutex
}

// 1. nMap -> total map tasks
// 2. nMapped -> finished map tasks
// 3. a lock is used to protect all variables
// 4. for each single Task and WorkerStat, 
// a lock is applied for fine grain purpose
type Master struct {
	tasks []Task
	ws []WorkerStat
	nMap int
	nReduce int
	nMapped int
	nReduced int
	lock sync.Mutex
}

// initialize workers with id
func (m *Master) Init(args *EmptyArgs, reply *InitReply) error {
	m.lock.Lock()
	reply.Id = len(m.ws)
	m.ws = append(m.ws, WorkerStat{time.Now().Unix(),true,sync.Mutex{}})
	m.lock.Unlock()
	reply.NReduce = m.nReduce
	return nil
}

// assign tasks to worker
func (m *Master) Assign(args *AssignArgs, reply *AssignReply) error {
	// find idle task and assign
	if !m.ws[args.Id].alive {
		log.Fatal("Master: dead assign")
	}
	for i, _ := range(m.tasks) {
		t := &m.tasks[i]
		t.lock.Lock()
		if t.status == IDLE {
			t.status = IN_PROGRESS
			t.worker = args.Id
			reply.File = t.file
			reply.Type = t.tipe
			t.lock.Unlock()
			return nil
		}
		t.lock.Unlock()
	}
	return nil
}

// finish task for worker
// if it is the last map task, add all reduce tasks to list
// after this, mark the task status to COMPLETE
func (m *Master) Finish(args *FinishArgs, reply *EmptyReply) error {
	if !m.ws[args.Id].alive {
		log.Fatal("Master: dead finish")
	}
	m.lock.Lock()
	if args.Type == TASK_MAP {
		m.nMapped++
		if m.nMapped == m.nMap {
			for i := 0; i < m.nReduce; i++ {
				m.tasks = append(m.tasks, Task{sync.Mutex{},0,-1,fmt.Sprintf("%v",i),TASK_REDUCE})
			}
		}
	} else {
		m.nReduced++
	}
	m.lock.Unlock()
	for i, _ := range(m.tasks) {
		t := &m.tasks[i]
		t.lock.Lock()
		if t.status == IN_PROGRESS && t.worker == args.Id {
			t.status = COMPLETED
			t.lock.Unlock()
			return nil
		}
		t.lock.Unlock()
	}
	return nil
}

func (m *Master) Alive(args *AliveArgs, reply *EmptyReply) error {
	w := &m.ws[args.Id]
	w.lock.Lock()
	w.lastseen = time.Now().Unix()
	w.lock.Unlock()
	return nil
}

// return nWorker
func (m *Master) NWorker(args *EmptyArgs, reply *NWorkerReply) error {
	reply.NWorker = len(m.ws)
	return nil
}

// 1. clean the files
// 2. clear task status and decrement completed count
func (m *Master) AfterDeath(i int) {
	var Type int
	if m.nMapped < m.nMap {
		Type = TASK_MAP
		for k := 0; k < m.nReduce; k++ {
			out := fmt.Sprintf("mr-%v-%v", i, k)
			f, _ := os.Create(out)
			f.Close()
		}
	} else {
		Type = TASK_REDUCE
		out := fmt.Sprintf("mr-out-%v", i)
		f, _ := os.Create(out)
		f.Close()
	}
	for j, _ := range m.tasks {
		t := &m.tasks[j]
		t.lock.Lock()
		if t.worker == i && (t.status == IN_PROGRESS || t.status == COMPLETED) && t.tipe == Type {
			if t.status == COMPLETED {
				m.lock.Lock()
				if t.tipe == TASK_MAP {
					m.nMapped--
				} else {
					m.nReduced--
				}
				m.lock.Unlock()
			}
			t.status = IDLE
			t.worker = -1
		}
		t.lock.Unlock()
	}
}

// killer thread
func (m *Master) Killer() {
	for {
		time.Sleep(time.Second * KILLER_PERIOD)
		for i, _ := range m.ws {
			ws := &m.ws[i]
			ws.lock.Lock()
			if ws.alive && time.Now().Unix() - ws.lastseen >= WORKER_TIMEOUT {
				fmt.Printf("Master: kill worker %v\n", i)
				ws.alive = false
				m.AfterDeath(i)
			}
			ws.lock.Unlock()
		}		
	}
}

//
// start a thread that listens for RPCs from worker.go
//
func (m *Master) server() {
	rpc.Register(m)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := masterSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrmaster.go calls Done() periodically to find out
// if the entire job has finished.
//
func (m *Master) Done() bool {
	return m.nReduce == m.nReduced
}

//
// create a Master.
// main/mrmaster.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeMaster(files []string, nReduce int) *Master {
	m := Master{}
	m.nMap = len(files)
	m.nReduce = nReduce
	m.nMapped = 0
	m.nReduced = 0
	go m.Killer()

	// set status to idle, set worker to -1
	for i := 0; i < len(files); i++ {
		m.tasks = append(m.tasks, Task{sync.Mutex{},0,-1,files[i],TASK_MAP})
	}
	m.server()
	return &m
}
