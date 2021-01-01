# Paper: Don't Settle for Eventual: Scalable Causal Consistency for Wide-Area Storage with COPS
## Introduction
1. ALPS: Availability, low Latency, Partition-tolerance, high Scalability
2. in this paper, we consider causal consistency with convergent conflict handling
3. COPS: Clusters of Order-Preserving Servers
4. two versions of COPS system: regular version, COPS; extended version, COPS-GT, which also provides `get transactions`
## ALPS Systems and Trade-Offs
### I. Desirable Properties
1. availability
2. low latency
3. partition tolerance: the data store continues to operate under network partitions
4. high scalability: adding N resources to the system increases aggregate throughput and storage capacity by O(N)
5. strong consistency
## Causal+ Consistency
~>: three rules defines
1. Execution Thread: a ~> b: a happens before b
2. Gets From: a `get` returns previous `put`
3. Transitivity: For operations a, b and c, if a ~> b and b ~> c, then a ~> c
### I. Definition
two properties: causal consistency and convergent conflict handling
1. causal consistency does not order concurrent operations
2. using a handler function h, which is associative and commutative, h(a,h(b,c)) = h(c,h(b,a))
3. later writer wins rule - Thomas's write rule: declares one of the conflicting write as having occurred later
### II. Causal+ vs. Other Consistency Models
1. consistency ranking: linearizability > sequential consistency > causal consistency > FIFO(PRAM) consisntency > per-key sequential consistency > eventual consistency
### III. Causal+ in COPS
1. two abstractions: a. versions. b. dependencies.
2. each replica in COPS always returns non-decreasing versions of a key. We refer to this as causal+ consistency's progressing property
### IV. Scalable Causality
1. log-exchange-based serialization inhibits replica scalability, as it relies on a single serialization point in each replica to establish ordering
2. in COPS: nodes in each datacenter are responsible for different partitions of the keyspace, but the system can track and enforce dependencies between keys stored on different nodes. COPS explicitly encodes dependencies in metadata associated with each key's version
## System Design of COPS
### I. Overview of COPS
1. COPS is a kv storage system designed to run across a small number of datacenters.
2. each local COPS cluster is set up as a linearizable kv store. Linearizable systems can be implemented scalably by partitioning the keyspace into N linearizable partitions. 
#### A. System Components
1. key-value store: a. each kv pair has associated metadata. In COPS, this metadata is a version number. In COPS=GT, it is both a version number and a list of dependencies (other keys and their respective versions) b. the kv store exports three additional operations as part of its kv interface: get_by_version, put_after, and dep_check c. For COPS-GT, the system keeps around old versions of kv pairs, not just the most recent put, to ensure that it can provide get transactions.
2. Client library: two main operations to applications, read via `get` or `get_trans`, and writes via `put`
#### B. Goals
1. minimize overhead of consistency-preserving replication
2. (COPS-GT) minimize space requirements
3. (COPS-GT) ensure fast `get_trans` operations
### II. The COPS Key-Value Store
1. <key, <version, value, deps>>
2. deps is a list of the version's 0 or more dependencies; each dependency is a <key, version>
3. for fault tolerance, each key is replicated across a small number of nodes using chain replication. Every key stored in COPS has one `primary node`, we term the set of primary nodes for a key across all clusters as the `equivalent nodes`
4. after the write completes locally, the primary node places it in a replication queue, from which it is sent asyncly to remote equivalent nodes
### III. Client Library and Interface
#### A. the client API
1. ctx_id <- createContext()
2. bool <- deleteContext(ctx_id)
3. bool <- put(key, value, ctx_id)
4. value <- get(key, ctx_id)
5. <values> <- get_trans(<keys>, ctx_id)
#### B. COPS-GT Client Library
1. client library in COPS-GT stores the client's context in a table of <key, version, deps> entries. Clients reference their context using a context ID (ctx_id). 
2. two concerns about the potential size of this causality graph: a. state requirements for storing these dependencies. b. the number of potential checks that must occur when replicating writes between clusters, in order to ensure causal consistency.
3. we term dependencies that must be checked the nearest dependencies.
4. the nearest dependencies are sufficient for the key-value store to provide causal+ consistency; the full dependency list is only needed to provide `get_trans` operations in COPS-GT
#### C. COPS Client Library
stores only <key, version> entries for less state and complexity
### IV. Writing Values in COPS and COPS-GT
#### A. Writes to the Local Cluster
when a client call put(key, val, ctx_id), the library computes the complete set of dependencies. deps, and identifies some of those dependency tuples as the value's nearest ones.
1. the library then calls `put_after` without the `version` argument. 
2. In COPS-GT, the library includes `deps` in the `put_after` call because dependencies must be stored with the value. 
3. In COPS, the library only needs to include `nearest` and does not include `deps`
#### B. Write Replication between Clusters
after a write commits locally, the primary storage node asynchronously replicates that write to its equivalent nodes in different clusters using a stream of `put_after` operations. Here, the primary node includes the key's version number in the `put_after` call. As with local `put_after` calls, the `deps` argument is included in COPS-GT, and not included in COPS. To ensure this property, a node that receives a `put_after` request from another cluster must determine if the value's `nearest` dependencies have already been satisfied locally. It uses `dep_check(key, version)`. 
### V. Reading Values in COPS
1. reads are satisfied in the local cluster
2. read can request either the latest version of the key or a specific older one. Requesting the latest version is equivalent to a regular single-key `get`; requesting a specific version is necessary to enable get transactions. Accordingly, `get_by_version` operations in COPS always request the latest version. Upon receiving response, the client library adds the <key, version[,deps]> tuple to the client context, and returns `value` to the calling code. The `deps` are stored only in COPS-GT, not in COPS
### VI. Get Transactions in COPS-GT
#### A. Get Transactions
get transactions algorithm has two rounds
1. the library issues n concurrent `get_by_version` operations to the local cluster, one for each key the client listed in `get_trans`. `get_by_version` returns a <value, version, deps> tuple, where `deps` is a list of keys and versions. The client library then examines every dependency entry <key, version>
2. for all keys that are not satisfied, the library issues a second round of concurrent `get_by_version` operations. The version requested will be the newest version seen in any dependency list from the first round. These versions satisfy all causal dependencies from the first round because they are >= the needed versions. Additionally, they do not introduce any new dependencies that need to be satisfied.
#### B. two important properties for the get transaction
1. the `get_by_version` requests will succeed immediately, as the requested version must already exist in the local cluster
2. the new `get_by_version` requests will not introduce any new dependencies.
## Garbage, Faults, and Conflicts
### I. Garbage Collection Subsystem
#### A. Version Garbage Collection
#### B. Dependency Garbage Collection
#### C. Client Metadata Garbage Collection
### II. Fault Tolerance
#### A. Client Failures
simply stops issuing new requests; no recovery is necessary. From a client's perspective, COPS's dependency tracking makes it easier to handle failures of other clients, by ensuring properties such as referential integrity.
#### B. Key-Value Node Failures
(didn't quite understand)
#### C. Datacenter Failures
1. any `put_after` operations that originated in the failed datacenter, but which were not yet copied out, will be lost.
2. the storage required for replication queues in the active datacenters will grow
3. in COPS-GT, dependency garbage collection cannot continue in the face of a datacenter failure
### III. Conflict Detection
COPS with conflict detection (COPS-CD) adds three new components to the system
1. all put operations carry with them `previous version` metadata, which indicates the most recent previous version of the key that was visible at the local cluster at the time of the write
2. all put operations now have an implicit dependency on that previous version, which ensures that a new version will only be written after its previous version. This implicit dependency entails an additional `dep_check` operation, though this has low overhead and always executes on the local machine
3. COPS-CD has an application specified convergent conflict handler that is invoked when a conflict is detected.
