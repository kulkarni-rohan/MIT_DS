# Questions and Answers
## Lecture 3
Describe a sequence of events that would result in a client reading stale data from the Google File System.
```
client cached the outdated chunk data, it's timeout after 60sec though
```
## Lecture 4
How does VM FT handle network partitions? That is, is it possible that if the primary and the backup end up in different network partitions that the backup will become a primary too and the system will run with two primaries?
```
No. The split-brain is avoid by using shared storage and test-and-set method. 
Case 1: When one found out it's not able to access the shared storage, it will commit suicide. 
Case 2.1: If both can access the shared storage and backup can't connect to primary, it will test-and-set, if it's network problem, the variable will be true, backup knows primary is live, then it will commit suicide.
Case 2.2: backup test-and-set successfully, it knows primary died, then it will take over. 
```
## Lecture 5
Consider the following code from the "incorrect synchronization" examples:
```
var a string
var done bool

func setup() {
	a = "hello, world"
	done = true
}

func main() {
	go setup()
	for !done {
	}
	print(a)
}
```
Using the synchronization mechanisms of your choice, fix this code so it is guaranteed to have the intended behavior according to the Go language specification. Explain why your modification works in terms of the happens-before relation.
```
// can use lock, channel and once.Do()
var a string
var lock sync.Mutex

func setup() {
	a = "hello, world"
	lock.Lock()
}

func main() {
	go setup()
    lock.Unlock()
	print(a)
}
```