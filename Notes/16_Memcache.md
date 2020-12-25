# Paper: Scaling Memcache at Facebook
## Overview
#### A. properties greatly influence our design
1. users consume an order of magnitude more content than they create
2. read operations fetch data from a variaty of sources such as MySQL databases, HDFS installations, and backend services
#### B. Query Cache
1. read: request value from `memcache` by providing a string key. If the item addressed by that key is not cached, the web server retrieves the data form the database or other backend service and populates the cache with the key-value pair
2. write: web server issues SQL statements to the database and then sends a delete request to `memcache` that invalidates any stale data. We choose to delete cached data instead of updating it because deletes are idempotent. `Memcache` is not the authoritative source of the data and is therefore allowed to evict cached data
#### C. Generic Cache
1. a more general key-value store
2. `memcache` provides no server-to-server coordination; it is an in-memory hash table running on a single server.
#### D. Design Goal
1. any change must impact a user-facing or operational issue. Optimizations that have limited scope are rarely considered
2. we treat the probability of treading transient stale data as a parameter to be tuned, similar to responsiveness. We are willing to expose slightly stale data in exchange for insulating a backend storage service from excessive load
## In a Cluster: Latency and Load
### I. Reducing Latency
1. hundreds of `memcached` servers in a cluster to reduce load on databases and other services
2. items are distributed across the `memcached` servers through consistent hashing
3. web servers have to routinely communicate with many `memcached` servers to satisfy a user request
4. all web servers communicate with every `memcached` server in a short period of time
#### A. Parallel Requests and Batching
construct a DAG representing the dependencies between data. A web server uses this DAG to maximize the number of items that can be fetched concurrently
#### B. Client-Server Communication
1. we embedded the complexity of the system into a stateless client rather than in the `memcached` server
2. client logic: a. a library that can be embedded into applications or as a standalone proxy named `mcrouter`. This proxy presents a `memcached` server interface and routes the requests/replies to/form other services; b. below
3. clients use UDP for get, don't recover from dropped/out-of-order packets; TCP for set and delete, sent through an instance of `mcrouter`, open TCP connection is expensive -> coalescing these connections saves resources, improves efficiency.
#### C. Incast Congestion
flow control -> sliding window -> grows slowly upon a successful request; shrinks when a request goes unanswered
### II. Reducing Load
#### A. Leases
lease is a per-key 64 bits token, it solves two problems
1. stale sets: occurs in concurrent updates
2. thundering herds: when a specific key undergoes heavy read and write activity. As the write repeatedly invalidates the recently set values, many reads default to the more costly path
##### Stale Values
a get request can return a lease token or data that is marked as stale. It can decide to go with stale data or wait for the latest value. Since cached value tends to be a monotonically increasing snapshot of the database, most applications can use a stale value without any changes.
#### B. Memcache Pools
1. wildcard pooll -> default
2. small pool -> keys that are accessed frequently but for which a cache miss is inexpensive
3. large pool -> infrequently accessed keys for which cache misses are prohibitively expensive
#### C. Replication within Pools
replicate a category of keys within a pool when
1. the application routinely fetches many keys simultaneously
2. the entire data set fits in one or two `memcached` servers
3. the request rate is much higher than what a single server can manage
### III. Handling Failures
two scales at which we must address failures
1. a small number of hosts are inaccessible due to a network or server failure -> auto remedian system, can take up to a few minutes, may cause cascading failures -> dedicate a small set of machines, named Gutter, to take over the responsibilities of a few failed servers -> when client receives no response to its get request, it assumes the server failed and issues the request again to Gutter pool. If this second request misses, the client will insert the appropriate kv pair into the Gutter machine after querying the database. Entries in Gutter expire quickly to obviate Gutter invalidations
2. a widespread outage that affects a significant percentage of the servers within the cluster
## In a Region: Replication
1. number of `memcached` servers increase -> incast congestion worsens
2. we therefore split our web and `memcached` servers into multiple `frontend clusters`.
3. these clusters, along with a storage cluster that contain the databases, define a `region`
4. this region architecture allows for smaller failure domains and a tractable network configuration. 
5. we trade replication of data for more independent failure domains, tractable network configuration, and a reduction of incast congestion
### I. Regional Invalidations
storage cluster is responsible for invalidating cached data to keep frontend clusters consistent with the authoritative versions. As an optimization, a web server that modifies data also sends invalidations to its own cluster rto provide read-after-write semantics for a single user request and reduce the amount of time stale data is present in its local cache.
#### A. Reducing Packet Rates
1. invalidation daemons batch deletes into fewer packets and send them to a set of dedicated servers running `mcrouter` instances in each frontend cluster
2. these `mcrouters` then unpack individual deletes from each batch and route those invalidations to the right `memcached` server co-located within the frontend cluster
#### B. Invalidation via Web Servers
disadvantages for web servers to broadcast invalidations
1. it incurs more packet overhead as web servers are less effective at batching invalidations than `mcsqueal` pipeline
2. it provides little recourse when a systemic invalidation problem arises such as misrouting of deletes due to a configuration error\
in contrast, embedding invalidations in SQL statements, allows replay invalidations lost/misrouted
### II. Regional Pools
1. definition: having multiple frontend clusters share the same set of `memcached` servers
2. one of the main challenge of scaling `memcache` within a region: deciding whether a key needs to be replicated across all frontend clusters or have a single replica per region
### III. Cold Cluster Warmup
1. definition: allowing clients in the "cold cluster" (i.e. the frontend cluster that has an empty cache) to retrieve data from the "warm cluster"(i.e. a cluster that has caches with normal hit rates) rather than the persistent storage.
2. care must be taken to avoid inconsistencies due to race condition. Memcached deletes support nonzero hold-off times that reject `add` operations for the specified hold-off time (2 sec by default). When a miss is detected in the cold cluster, the client re-requests the key from the warm cluster and adds it into the cold cluster. While there is still a theoretical possibility that deletes get delayed more than two seconds, this is not true for the vast majority of the cases. The operational benefits of cold cluster warmup far outweigh the cost of rare cache consistency issues. We turn it off once the cold cluster’s hit rate stabilizes and the benefits diminish.
## Across Regions: Consistency
advantages to a broader geographic placement of data centers:
1. reduce latency
2. mitigate the effects of events such as natural disasters or massive power failures
3. new locations can provide cheaper power and other economic incentives\
structure
1. one region to hold the master databases and the other regions to contain read-only replicas
2. rely on MySQL’s replication mechanism to keep replica databases up-to-date with their masters. 
3. biggest challenge: replica databases may lag behind the master database
#### A. Writes from a Master Region
1. storage cluster to invalidate data via daemons 
2. we implement `mcseqeal` after scaling to multiple regions
#### B. Writes from a Non-Master Region
1. we employ `remote marker` to minimize the probability of reading stale data. The presence of the marker indicates that data in the local replica database are potentially stale and the query should be redirected to the master region
2. when a web server wishes to update data that affects a key k, that server a. sets a remote marker rk in the region; b. performs the write to the master embedding k and rk to be invalidated in the SQL statement; c. deletes k in the local cluster
3. we implement remote markers by using a regional pool. Note this mechanism may reveal stale informaiton during concurrent modifications to the same key as one operation may delete a remote marker that should remain present for another in-flight operation. It is worth highlighting that our usage of `memcache` for remote markers departs in a subtle way from caching results. As a cache, deleting or evicting keys is always a safe action; it may induce more load on databases, but does not impair consistency. In contrast, the presence of a remote marker helps distinguish whether a non-master database holds stale data or not. In practice, we find both the eviction of remote markers and situations of concurrent modification to be rare.
#### C. Operational Consideration
Inter-region communication is expensive. The aforementioned system for managing deletes in Section 4.1 is also deployed with the replica databases to broadcast the deletes to memcached servers in the replica regions. Databases and mcrouters buffer deletes when downstream components become unresponsive. A failure or delay in any of the components results in an increased probability of reading stale data. The buffered deletes are replayed once these downstream components are available again. The alternatives involve taking a cluster offline or over-invalidating data in frontend clusters when a problem is detected. These approaches result in more disruptions than benefits given our workload.