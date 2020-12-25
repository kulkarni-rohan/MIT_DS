# Paper: Resilient Distributed Datasets: A Fault-Tolerant Abstraction for In-Memory Cluster Computing
## Resilient Distributed Datasets(RDDs)
### I. RDD Abstraction
1. read-only, partitioned collection of records
2. only be created through deterministic operations on either a. data in stable storage b. other RDDs
3. transformation: `map`, `filter`, `join`
4. a program cannot reference an RDD that it cannot reconstruct after a failure
5. users can control a. persistence, choose storage strategy (e.g. in-memory storage) b. partitioning, ask that an RDD's elements be partitioned across machines based on a key in each record
### II. Spark Programming Interface
1. can start by defining one or more RDDs through transformations on data in stable storage
2. use these RDDs in `actions`, include `count` `collect` `save`
3. Spark computes RDDs lazily the first time they are used in an action
4. programmers can call a `persist` method to indicate which RDDs they want to reuse in future operations, default in memory
#### Example: Console Log Mining
```
lines = spark.textFile("hdfs://...")
errors = lines.filter(_.startsWith("ERROR"))
errors.persist()
```
```
// Count errors mentioning MySQL:
errors.filter(_.contains("MySQL")).count()
// Return the time fields of errors mentioning
// HDFS as an array (assuming time is field
// number 3 in a tab-separated format):
errors.filter(_.contains("HDFS"))
.map(_.split(’\t’)(3))
.collect()
```
### III. Advantage of the RDD Model
1. RDD can only be created ("written") through coarse-grained transformations. Cons: a. Restricts RDDs to applications that perform bulk writes. Pros: a. allows for more efficient fault tolerance. b. RDDs do not need to incur the overhead of checkpointing, as they can be recovered using lineage. c. the lost partitions of an RDD need to be recomputed upon failure, and they can be recomputed in parallel on different nodes, without having to roll back the whole programme
2. the immutable nature lets a system mitigate slow nodes by running backup copies of slow tasks as in MR. Backup tasks would be hard to implement with DSM, as the two copies of a task would access the same memory locations and interfere with each other's updates
3. RDD provide two other benefits over DSM. a. in bulk operations on RDDs, a runtime can schedule tasks based on data locality to improve performance. b. Partitions do not fit in RAM can be stored on disk and will provide similar performance to current data-parallel systems.
### IV. Applications Not Suitable for RDDs
1. suitable for batch applicaitons
2. not suitable for applications that make async fine-grained updates to shared state, such as a storage system for a web applicaiton or an incremental web crawler.
## Spark Programming Interface 
1. driver program connects to a cluster of workers. The driver defines one or more RDDs and invokes actions on them. Spark code on the driver also tracks the RDDs' lineage. The workers are long-lived processes that can store RDD partitions in RAM across operations
2. RDDs themselves are statically typed objects parametrized by an element type
### I. RDD Operations in Spark
Table 2
### II. Example Application
#### A. Logistic Regression
```
val points = spark.textFile(...)
.map(parsePoint).persist()
var w = // random initial vector
for (i <- 1 to ITERATIONS) {
val gradient = points.map{ p =>
p.x * (1/(1+exp(-p.y*(w dot p.x)))-1)*p.y
}.reduce((a,b) => a+b)
w -= gradient
}
```
#### B. PageRank
```
// Load graph as an RDD of (URL, outlinks) pairs
val links = spark.textFile(...).map(...).persist()
var ranks = // RDD of (URL, rank) pairs
for (i <- 1 to ITERATIONS) {
// Build an RDD of (targetURL, float) pairs
// with the contributions sent by each page
val contribs = links.join(ranks).flatMap {
(url, (links, rank)) =>
links.map(dest => (dest, rank/links.size))
}
// Sum contributions by URL and get new ranks
ranks = contribs.reduceByKey((x,y) => x+y)
.mapValues(sum => a/N + (1-a)*sum)
}
```
## Representing RDDs
### I. represent each RDD through a common interface that exposes 5 pieces of information
1. a set of `partitions`, which are atomic pieces of the dataset
2. a set of `dependencies` on parent RDDs
3. a function for computing the dataset based on its parents
4. metadata about its partitioning scheme and data placement
5. ???(i didn't find it)
### II. N/W Dependencies
1. Narrow Dependencies: each partition of the parent RDD is used by at most one partition of the child RDD
2. Wide Dependencies: multiple child partitions may depend on it
### III. Why N/W is useful
1. narrow dependencies allow for pipelined execution on one cluster node, which can compute all the parent partitions; wide dependencies require data from all parent partitions to be available and to be shuffled across the nodes using a MR like operation
2. recovery after a node failure is more efficient with a narrow dependency, as only the lost parent partitions need to be recomputed, and they can be recomputed in parallel on different nodes; in a lineage graph with wide dependencies, a single failed node might cause the loss of some partition from all the ancestors of an RDD, requiring a complete re-execution
### IV. some RDD implementation
1. HDFS files
2. map: RDD -> MappedRDD
3. union: two RDD -> an RDD
4. sample: similar with mapping
5. join: joining two RDDs may lead to either two narrow dependencies, two wide dependencies or mix
## Implementation
### I. Job Scheduling
1. take into account which partitions of persistent RDDs are available in memory
2. whenever a user runs an action on an RDD, the scheduler examines that RDD's lineage graph to build a DAG of `stages` to execute.
3. Each stage contains as many pipelined transformatinos with narrow dependencies as possible
4. The boundaries of the stages are the shuffle operations required for wide dependencies, or any already computed partitions that can short circuit the computation of a parent RDD
#### Details
1. assign tasks based on data locality using delay scheduling
2. for wide dependencies, we currently materialize intermediate records on the nodes holding parent partitions to simplify fault recovery, much like MR materializes map outputs
3. task fail -> rerun it on another node as long as parents are still avaiable; otherwise, we resubmit tasks to compute the missing partitions in parallel. 
4. we do not yet tolerate scheduler failures
5. although driver coordinates the computations, we are experimenting with letting tasks on the cluster call the `lookup` operation, which provides random access to elements of hash-partitioned RDDs by key.
### II. Interpreter Integration
two changes to the interpreter
1. Class Shipping: let the worker nodes fetch the bytecode for the classes created on each line, we made the interpreter serve these classes over HTTP
2. Modified Code Generation: Normally, the singleton object created for each line of code is accessed through a static method on its corresponding class. This means that when we serialize a closure referencing a variable defined on a previous line, such as Line1.x in the example above, Java will not trace through the object graph to ship the Line1 instance wrapping around x. Therefore, the worker nodes will not receive x. We modified the code generation logic to reference the instance of each line object directly
### III. Memory Management
1. three options for storage of persistent RDDs: a. in-memory, deserialized; b. in-memory, serialized; c. on-disk storage
2. to manage the limited memory, we use LRU eviction policy -> evict a partition from the least recently accessed RDD
3. each instance of Spark on a cluster currently has its own separate memory space
### IV. Support for Checkpointing
1. checkpointing is useful for RDDs with long lineage graphs containing wide dependencies
2. Spark provides an API for checkpointing (a REPLICATE flag to `persist`), leaves the decision of which data to checkpoint to the user
3. read-only nature of RDDs makes them simpler to checkpoint than general shared memory. Because consistency is not a concern.