# Paper: Mapreduce
## Introduction
purposes:
1. parallelization
2. fault-tolerance
3. data distribution
4. load balancing
## Programming Model
### I. Map
1. written by user
2. takes an input pair and produces a set of intermediate key/value pairs
### II. Reduce
1. accepts an intermediate key I and a set of values for that key
2. merges together those values to form a possibly smaller set of values, typically just 0 or 1 output value is produced per `Reduce` invocation.
### III. type
```
map     (k1, v1)        ->  list(k2, v2)
reduce  (k2, list(v2))  ->  list(v2)
```
## Implementation
Many different implementations of the MapReduce interface are possible. The right choice depends on the environment. For example, one implementation may suitable for a small shared-memory machine, another for a large NUMA multi-processor, and yet another for an even large collection of networked machines.
### I. Execution Overview
1. split input for `map`:  split input files into `M` pieces of typically 16MB to 64MB perpiece.
2. assign task to worker: `M` map tasks and `R` reduce tasks. The master picks idle workers and assigns each one a map task or a reduce task
3. `map`: map worker reads the contents of the corresponding input split. It parses key/value pairs and passes each pair to the user-defined `Map` function. The intermediate key/value pairs produced by `Map` are buffered in memory.
4. buffer back and split work for `reduce`: periodically, the buffered pairs are written to local disk (of worker), partitioned into R regions by the partitioning function. The locations of these buffered pairs are passed back to the master.
5. rpc + sort: when a reduce worker is notified by the master about these locations, it uses rpc to read the buffered data from map workers. When a reduce worker has read all intermediate data, it sorts it by the intermediate keys. 
6. `reduce`: reduce worker iterates over the sorted intermediate data and for each unique key encountered, it passes the key to the user's `Reduce` function. Output is appended to a final output file for this reduce partition.
7. return: when all tasks completed, the master wakes up the user program, return back to user code.
### II. Master Data Structures
1. map task and reduce task: state (idle, in-progress, completed) and identiry of the worker machine (for non-idle tasks)
2. for each complted map task: locations and sizes of the R intermediate file regions. The information is pushed incrementally to in-progress reduce workers.
### III. Fault Tolerance
#### A. Worker Failure
master pings every worker periodically. \
No response for a certain amount of time:
1. both completed and in-progress map tasks are reset to idle
2. reduce workers executing these are notified
#### B. Master Failure
1. write periodic checkpoints of the master data structures
2. only one master -> failure is unlikely -> aborts MapReduce computation
#### C. Semantics in the Presence of Failures
1. if `map` and `reduce` are deterministic -> same output as non-faulting sequential execution
2. non-deterministic -> we provide weaker but still reasonable semantics -> output of reduce task R1 = output for R1 produced by a sequential execution of the non-deterministic program. However, output of R2 may correspond to the output for a different sequential execution of the non-deterministic program.
#### D. Locality
