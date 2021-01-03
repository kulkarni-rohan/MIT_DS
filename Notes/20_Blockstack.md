# Blockstack Technical Whitepaper
## System Architecture
#### A. Design Goals
1. Decentralized Naming & Discovery
2. Decentralized Storage
3. Comparable Performance
#### B. Design Decisions
1. Survive Failures of Underlying Blockchains
2. Keep Complexity and Logic Outside of Blockchains
3. Scalable Index for Global Data
### I. Blockstack Layers
one layer (the blockchain layer) in the control plane and two layers (the peer network and data-storage) in the data plane
#### A. Layer 1: Blockchain
1. two purposes: a. storage medium for operations b. consensus on the order in which the operations were written
2. the blockchain layer also includes a virtualchain, which defines new operations without requiring changes to the underlying blockchain. Nodes of the underlying blockchains are not aware of this layer
3. virtualchains are like virtual machines. Virtualchain operations are encoded in valid blockchain transactions as additional metadata
#### B. Layer 2: Peer Network
1. zone files: storing routing information in the discovery layer
2. users do not need to trust the discovery layer because it can be verified by checking records in the control plane
#### C. Layer 3: Storage
1. Users do not need to trust the storage layer and can verify their integrity in the control plane
## BNS: Blockchain Naming System
solves Zooko's Triangle: a. unique; b. human-readable; c. decentralized
### I. BNS Operations
1. names are owned by cryptographic addresses of the underlying blockchain and their associated private keys
2. a user `preorders` and then `registers` a name in a two-phase commit process
3. once a name is registered, a user can `update` the name-value pair by sending an update transaction and uploading the name-value binding. Name `transfer` operations simply change the address that is allowed to sign subsequent transactions, while `revoke` operations disable any further operations for names
4. in BNS, names are organized into `namespaces`, which are the functional equivalent of top-level domains in DNS
5. the information for top-level domains (namespaces) is registered on a `root blockchain`
6. to resolve a name, the end-user makes a query to the local BNS server running inside her trust zone. The local BNS server looks at the respective blockchain record and fetches the respective zone file from an external source, like a peer network.
#### A. Pricing Functions
1. the price of a name drops with an increase in name length 
2. introducint non-alphabetic characters in names also drops the price
### II. Public Keys in BNS
1. a blockchain can be used as a global distribution mechanism for public keys and digital certificates
2. BNS already provides public key associations with domain names and all domains, by default, get certificates. In BNS, domain names can serve as memorable identifiers for public keys
## Virtualchain
1. virtualchain is a virtual blockchain for creating arbitrary state machines on top of already-running blockchains.
2. blockchain provides a totally-ordered, tamper-resistant log of state transitions
3. by using the blockchain as a shared communication channel, these applications can then bootstrap global state in a secure, decentralized manner, since every node on the network can independently construct the same state
#### A. two key challenges to using blockchains as a building block for decentralized applications and service
1. a blockchain can fail, can go offline, or its consensus mechanism can become "centralized" by falling under the de facto control of a single entity.
2. the application's log can be forked and corrupted if the underlying blockchain forks
### I. Design of Virtualchains
#### A. Consistency Model
a variant  of Nakamoto consensus, which allows concurrent leaders. Appending conflicting blocks creates blockchain forks, which peers resolve using a proof-of-work metric.
#### B. Consensus Hashes
A consensus hash is a cryptographic hash that each node calculates at each block. It is derived from the accpeted state transitions in the last-processed block, and a geometric series of prior-calculated consensus hashes.
#### C. Fast Queries
1. we use a protocol for fast queries that is useful for creating "lightweight nodes" that do not need blockchain or state replicas. Instead, they can query highly-available but untrusted "full nodes" as needed
2. for fast queries, application users obtain CH(n) from a trusted node, such as one running on the same host. A user can then use this trusted CH(n) to query previous state transitions from untrusted nodes in a logarithmic amount of time and space
#### D.  Blockchain Fork Detection and Recovery
1. to detect deep chain reorgs, a node runs multiple processes that subscribe to a geometric series of prior block heights.
### II. Cross-chain Migration
1. virtualchains can survice the failure of an underlying blockchain by migrating state to another blockchain.
2. doing so requires announcing a future block until which the current blockchain will be valid, and then executing a two-step commit to bind the existing state to the new blockchain.
## Atlas Network
#### A. Challenges with Peer Networks
1. scalability: in unstructured peer networks, as the number of participant peers increases, the number of messages exchanged for a lookup grows
2. performance: reads and writes on peer networks have very variable latency depending on the underlying design, but in most cases, the worst-case performance of lookups is unacceptable; a request can needlessly bounce through several high latency network links before being handled
3. reliability: public peer networks allow anyone to write to them
4. junk data writes: without some rate-limiting or access-control mechanism, peer networks have no way to limit the amount of data inserted
5. node eclipse attack: in structured peer networks, an attacker can take over the neighbors of all nodes storing a particular key/value pair and effectively censor nodes/keys from the network
#### B. details
1. all Atlas nodes maintain a 100% state replica, and they organize into an unstructured overlay network.
##### i. Network Partitions
1. the Atlas network is more reliable than the previous DHT network. Less network partition problem
##### ii. Node Recovery
1. Atlas nodes can recover from failures on their own.
2. if the local index of the Atlas data becomes corrupt, the nodes can reconstruct it from the blockchain data. Nodes can also re-fetch all zone files in case of a data loss. 
## Gaia: Decentralized Storage
1. users can log in to apps and services by using blockchain-based decentralized identity and save data generated by apps/services on storage backends owned by the user.
2. we treat cloud storage providers as "dumb drives" and store encrypted and/or signed data on them
3. writing the data involves signing and replicating the data
4. reading the data involves fetching the zone file and data, verifying that hash(zonefile) matches the hash in the blockchain, and verifying the data's signature with the user's public key. THis allows for writes to be as fast as the signature algorithm and underlying storage system alllow, since updating the data does not alter the zone file and thus does not require any blockchain transactions.
#### A. Performance
comparable to traditional cloud providers
#### B. System Scalability
the storage layer of our architecture is not a scalability bottleneck.
## Conclusion 
Blockstack provides a full stack to developers for building decentralized applications including services dor identity, discovery, and storage
