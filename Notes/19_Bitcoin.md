# Paper: Bitcoin: A Peer-to-Peer Electronic Cash System
## Transactions
1. electronic coin: a chain of digital signatures
2. problem: payee can't verify that one of the owners did not double-spend the coin.
3. we need a way for the payee to know that the previous owners did not sign any earlier transactions. The only way to confirm the absence of a transaction is to be aware of all transactions. To accomplish this without a trusted party, transactions must be publicly announced, and we need a system for participants to agree on a single history of the order in which they were received. The payee needs proof that that at the time of each transaction, the majority of nodes agreed it was the first received
## Timestamp Server
1. take a hash of a block of items to be timestamped and widely publishing the hash.
2. the timestamp proves that the data must have existed at the time in order to get into the hash.
3. each timestamp includes the previous timestamp in its hash, forming a chain, with each additional timestamp reinforcing the ones before it.
## Proof-of-Work
1. the proof-of-work involves scanning for a value that when hashed, such as SHA-256, the hash begins with a number of zero bits.
2. the average work required is exponential in the number of zero bits required and can be verified by executing a single hash.
3. we implement the proof-of-work by incrementing a nonce in the block until a value is found that gives the block's hash the required zero bits.
4. the proof-of-work also solves the problem of determining representation in majority decision making. It's one-CPU-one-vote.
5. the majority decision is represented by the longest chain, which has the greatest proof-of-work effort invested in it.
## Network
### I. the steps to run the network
1. new transactions are broadcast to all nodes
2. each node collects new transactions into a block
3. each node works on finding a difficult proof-of-work for its block
4. when a node finds a proof-of-work, it broadcasts the block to all nodes
5. nodes accept the block only if all transactions in it are valid and not already spent
6. nodes express their acceptance of the block by working on creating the next block in the chain, using the hash of the accepted block as the previous hash.
### II. few rules
1. nodes always consider the longest chain to be the correct one and will keep working on extending it.
2. if two nodes broadcast different versions of the next block simultaneously, some nodes may receive one or the other first. In that case, they work on the first one they received.
## Incentive
1. the first transaction in a block is a special transaction that starts a new coin owned by the creator of the block.
2. the incentive can also be funded with transaction fees. If the output value of a transaction is less that its input value, the difference is a transaction fee that is added to the incentive value of the block containing the transaction. Once a predetermined number of coins have entered circulation, the incentive can transition entirely to transaction fees and be completely inflation free
3. the incentive may help encourage nodes to stay honest.
## Reclaiming Disk Space
1. Once the latest transaciton in a coin is buried under enough blocks, the spent transactions before it can be discarded to save disk space
2. to facilitate this without breaking the block's hash, transactions are hashed in a Merkle Tree, with only the root included in the block's hash. 
## Simplified Payment Verification
1. it is possible to verify payments without running a full network node. A user only needs to keep a copy of the block headers of the longest proof-of-work chain, which he can get by querying network nodes until he's convinced he has the longest chain, and obtain the Merkle branch linking the transaction to the block it's timestamped in.
2. verification is reliable as long as honest nodes control the network, but is more vulnerable if the network is overpowered by an attacker -> how to protect against this: accept alers from network nodes when they detect an invalid block, prompting the user's software to download the full block and alerted transactions to confirm the inconcistency. 
## Combining and Splitting Value
1. transactions contain multiple inputs and outputs. Normally there will be either a single input from a larger previous transaction or multiple inputs combining smaller amounts, and at most two outputs: one for the payment, and one returning the change, if any, back to the seller
2. it should be noted that fan-out, where a transaction depends on several transactions, and those transactions depend on many more, is not a problem here. There is never the need to extract a complete standalone copy of a transaction's history
## Privacy
1. tranditionally: limiting access to information; new model: anonymous
2. a new key pair should be used for each transaction to keep them from being linked to a common owner. Some linking is still unavoidable with multi-input transactions, which necessarily reveal that their inputs were owned by the same owner. The risk is that if the owner of a key is revealed, linking could reveal other transactions that belonged to the same owner.