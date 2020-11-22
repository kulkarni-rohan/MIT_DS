package mr

import "fmt"
import "log"
import "net/rpc"
import "hash/fnv"
import "os"
import "time"
import "io/ioutil"
import "encoding/json"
import "bufio"


//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

type MyWorker struct {
	id int
	nReduce int
	nWorker int
	out string
}

var myWorker MyWorker
//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

// return the content of file in string
func map_read(file *string) *string {
	ofile, _ := os.Open(*file)
	defer ofile.Close()
	b, _ := ioutil.ReadAll(ofile)
	content := string(b)
	return &content
}

// append kv to corresponding intermediate file
func map_write(kv *KeyValue) {
	k := ihash(kv.Key) % myWorker.nReduce
	b, _ := json.Marshal(&kv)
	midfile := fmt.Sprintf("mr-%v-%v", myWorker.id, k)
	f, _ := os.OpenFile(midfile, os.O_APPEND|os.O_WRONLY, 0666)
	f.WriteString(string(b)+"\n")
	f.Close()
}

// read the intermedia file and sort by keys using map
func reduce_read(rnum *string, nworker int) *map[string][]string {
	m := make(map[string][]string)
	for i := 0; i < nworker; i++ {
		file := fmt.Sprintf("mr-%v-%v", i, *rnum)
		ofile, _ := os.Open(file)
		scanner := bufio.NewScanner(ofile)
		var tmp KeyValue
		for scanner.Scan() {
			json.Unmarshal([]byte(scanner.Text()), &tmp)
			m[tmp.Key] = append(m[tmp.Key], tmp.Value)
		}
		ofile.Close()
	}
	return &m
}

// write to final output file, it's one file per worker
func reduce_write(k *string, res *string) {
	f, _ := os.OpenFile(myWorker.out, os.O_APPEND|os.O_WRONLY, 0666)
	f.WriteString(fmt.Sprintf("%v %v\n", *k, *res))
	f.Close()
}

//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue, reducef func(string, []string) string) {
	call_init()
	go imalive()
	for i := 0; i < myWorker.nReduce; i++ {
		f, _ := os.Create(fmt.Sprintf("mr-%v-%v", myWorker.id, i))
		f.Close()
	}
	f, _ := os.Create(myWorker.out)
	f.Close()
	for {
		file, tipe := call_assign()
		if file != "" {
			if tipe == TASK_MAP {
				kvs := mapf(file, *map_read(&file))
				for _, kv := range kvs {
					map_write(&kv);
				}
			} else {
				nWorker := call_nworker()
				m := reduce_read(&file, nWorker)
				for k, v := range *m {
					res := reducef(k,v)
					reduce_write(&k, &res)
				}
			}
			call_finish(tipe)
			time.Sleep(time.Second)
		} else {
			time.Sleep(time.Second)
		}
	}
}

// for debugging
func typeName(Type int) string {
	if Type == TASK_MAP {
		return "Map"
	} else {
		return "Reduce"
	}
}

func imalive() {
	for {
		time.Sleep(IMALIVE_PERIOD * time.Second)
		call_alive()
	}
}

// worker initialization
func call_init() {
	args := EmptyArgs{}
	reply := InitReply{}
	call("Master.Init", &args, &reply)
	myWorker.id = reply.Id
	myWorker.nReduce = reply.NReduce
	myWorker.out = fmt.Sprintf("mr-out-%v", reply.Id)
	fmt.Printf("Worker: init with id %v\n", myWorker.id)
}

// worker ask master for assignment of task
func call_assign() (string, int) {
	args := AssignArgs{myWorker.id}
	reply := AssignReply{}
	call("Master.Assign", &args, &reply)
	if reply.File != "" {
		fmt.Printf("Worker %v: assigned %v task %v\n", myWorker.id, typeName(reply.Type), reply.File)
	}
	return reply.File, reply.Type
}

// worker reports the finish of task
func call_finish(tipe int) {
	args := FinishArgs{myWorker.id, tipe}
	reply := EmptyReply{}
	fmt.Printf("Worker %v: finished %v\n", myWorker.id, typeName(args.Type))
	call("Master.Finish", &args, &reply)
}

// worker ask master how many workers are present
func call_nworker() int {
	args := EmptyArgs{}
	reply := NWorkerReply{}
	call("Master.NWorker", &args, &reply)
	return reply.NWorker
}

// worker tell master that it is still alive, don't kill it
func call_alive() {
	args := AliveArgs{myWorker.id}
	reply := EmptyReply{}
	call("Master.Alive", &args, &reply)
}

//
// send an RPC request to the master, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := masterSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
