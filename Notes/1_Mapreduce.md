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
### IV. Locality
1. Network Bandwidth is scarce.
2. GFS devides each file into 64MB blocks and stores several copies (typically 3) of each block on different machines.
3. attempts to schedule a map task on a machine that contains a replica of the corresponding input data.
4. failing that, it attampts to schedule a map task near a replica (e.g. on the same network switch)
### V. Task Granularity
1. ideally, M and R should be much larger than the number of worker -> improves dynamic load balancing and speeds up recovery.
2. time: O(M + R) decisions. space: O(M * R) state for each M-R pair.
3. R is often constrained by users because the output of each reduce task ends up in a separate output file.
4. choose M: 16MB - 64MB for each task; choose R: small multiple of the number of worker machines.
### VI. Backup Tasks
eliminate "stragglers": when a MapReduce is close to completion, the master schedules backup executions of the remaining in-progress tasks. The task is marked as completed whenever either the primary or the backup execution completes.
## Refinements
### I. Partitioning Function
1. default: hash(key) mod R, result in well-balanced partitions
2. if you want urls with the same hostname to be in a same file: hash(Hostname(url)) mod R
### II. Ordering Guarantees
within a given partition, the intermediate key/value pairs are processed in increasing key order.
### III. Combiner Function
1. optional
2. executed after `map`
3. code is same as `reduce`, the only difference is: `combiner` writes in intermediate file, `reduce` writes in final output file.
### IV. Input and Output Types
1. user can define `reader` interface to define the input format.
2. similar for output.
### V. Side-effects
1. users may found it convenient to produce auxiliary files as additional outputs from their map and reduce operators.
2. "we" rely on the application writer to make such side-effects atomic and idempotent.
3. "we" do not provide support for atomic two-phase commits of multiple output files produced by a single task. Therefore, tasks that produce multiple output files with cross-file consistency requirements shoudl be deterministic.
### VI. Skipping Bad Records
1. each worker process installs a signal handler that catches segmentation violations and bus errors.
2. if user code generates a signal, the signal handler sends a "last gasp" UDP packet that contains the sequence number to the MapReduce master
3. when the master has seen more than one failure on a particular record, it skips that.
### VII. Local Execution
"we" have developed an implementation of the MapReduce library that sequentially executes all the work for a MapReduce operation on the local machine.
### VIII. Status Information
master runs an internal HTTP server and exports a set of status pages for human consumption
### IX. Counters
1. MP library provides a counter facility to count occurrences of various events.
2. some counter values are auto maintained by MP library (i.e. # of input k/v processed, # of output k/v produced)
## Conclusion
### I. Why we succeeded?
1. easy to use, hidden details
2. large variety of problems are easily expressible as MP computations
3. "we" developed an implementation of MP that scales to large clusters of machines comprising thousands of machines.
### II. What we learnt?
1. restricting the programming model makes it easy to parallelize and distribute computations and to make such computations fault-tolerant
2. network bandwidth is a scarce resource
3. redundant execution can be used to reduce the impact of slow machines, and to handle machine failures and data loss.