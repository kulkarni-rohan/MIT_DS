# Certificate Transparency
## What is Certificate Transparency
making the issuance and existence of SSL certificates open to scrutiny by domain owners, CAs, and domain users. Specifically, Certificate Transparency has three main goals:
1. Make it impossible (or at least very difficult) for a CA to issue a SSL certificate for a domain without the certificate being visible to the owner of that domain.
2. Provide an open auditing and monitoring system that lets any domain owner or CA determine whether certificates have been mistakenly or maliciously issued.
3. Protect users (as much as possible) from being duped by certificates that were mistakenly or maliciously issued.\
Certificate Transparency satisfies these goals by creating an open framework for monitoring the TLS/SSL certificate system and auditing specific TLS/SSL certificates. This open framework consists of three main components, which are described below.
### I. Certificate Logs
Certificate logs are simple network services that maintain cryptographically assured, publicly auditable, append-only records of certificates. 
### II. Monitors
Monitors are publicly run servers that periodically contact all of the log servers and watch for suspicious certificates.
### III. Auditors
Auditors are lightweight software components that typically perform two functions.
1. they can verify that logs are behaving correctly and are cryptographically consistent. If a log is not behaving properly, then the log will need to explain itself or risk being shut down. 
2. they can verify that a particular certificate appears in a log. This is a particularly important auditing function because the Certificate Transparency framework requires that all SSL certificates be registered in a log. If a certificate has not been registered in a log, it’s a sign that the certificate is suspect, and TLS clients may refuse to connect to sites that have suspect certificates. 
## How Certificate Transparency Works
### I. Basic Log Features
#### A. 3 important qualities
1. append-only
2. cryptographically assured
3. publicly auditable
#### B. other
Every certificate log must publicly advertise its URL and its public key (among other things). Anyone can interact with a log via HTTPS GET and POST messages.
### II. Basic Log Operations
Anyone can submit a certificate to a log, although most certificates will be submitted by certificate authorities and server operators. When someone submits a valid certificate to a log, the log responds with a signed certificate timestamp (SCT), which is simply a promise to add the certificate to the log within some time period. The time period is known as the maximum merge delay (MMD).\
Certificate Transparency supports three methods for delivering an SCT with a certificate. Each is described below. 
#### A. X.509v3 Extension
Figure 1
#### B. TLS Extension
Figure 2
#### C. OCSP Stapling
Figure 2
### III. Basic Monitor and Auditor Operations
### IV. Typical System Configuration
#### A. Monitors
1. Monitors watch for suspicious certificates in logs, such as illegitimate or unauthorized certificates, unusual certificate extensions, or certificates with strange permissions (for example, CA certificates). 
2. Monitors also verify that all logged certificates are visible in the log. They do this by periodically fetching all the new entries that have been added to a log. 
3. As a result, most monitors have complete copies of the logs they monitor. If a log goes offline for a prolonged period of time, and a monitor has a copy of the log, then the monitor could act as a backup read-only log and provide log data to other monitors and auditors that are trying to query the log.
#### B. Auditors
1. Auditors verify the overall integrity of logs. Some auditors can also verify whether a particular certificate appears in a log. They do this by periodically fetching and verifying log proofs. Log proofs are signed cryptographic hashes a log uses to prove it’s in good standing. Every log must provide their log proofs on demand.
2. Auditors can use log proofs to verify that a log’s new entries have always been added to the log’s old entries, and that nobody has ever corrupted a log by retroactively inserting, deleting, or modifying a certificate. 
3. Auditors can also use log proofs to prove that a particular certificate appears in a log. This is particularly important because the Certificate Transparency framework requires that all SSL certificates be entered into a log. If a TLS client determines (via an auditor) that a certificate is not in a log, it can use the SCT from the log as evidence that the log has not behaved correctly.
#### C. Consistency
While log proofs allow an auditor or a monitor to verify that their view of a particular log is consistent with their past views, they also need to verify that their view of a particular log is consistent with other monitors and auditors. To facilitate this verification, auditors and monitors exchange information about logs through a gossip protocol. This asynchronous communication path helps auditors and monitors detect forked logs.
### V. Typical System Configuration
In a typical configuration, a CA runs a monitor and a client (browser) runs an auditor (see figure 3). This configuration simplifies the messaging that’s necessary for monitoring and auditing, and it lets certificate authorities and clients develop monitoring and auditing systems that meet the specific needs of their customers and users.
#### A. Certificate Issuance
A CA obtains an SCT from a log server and incorporates the SCT into the SSL certificate using an X.509v3 extension (for more details on this process, see figure 1). The CA then issues the certificate (with the SCT attached) to the server operator. This method requires no server updates (all servers currently support X.509v3 extensions), and it lets server operators manage their certificates the same way they’ve always managed their SSL certificates.
#### B. TLS Handshake
During the TLS handshake, the TLS client receives the SSL certificate and the certificate’s SCT. As usual, the TLS client validates the certificate and its signature chain. In addition, the TLS client validates the log’s signature on the SCT to verify that the SCT was issued by a valid log and that the SCT was actually issued for the certificate (and not some other certificate). If there are discrepancies, the TLS client may reject the certificate. For example, a TLS client would typically reject any certificate whose SCT timestamp is in the future.
#### C. Certificate Monitoring
Most monitors will likely be operated by certificate authorities. This configuration lets certificate authorities build efficient monitors that are tailored to their own specific monitoring standards and requirements.
#### D. Certificate Auditing
Most auditors will likely be built into browsers. In this configuration, a browser periodically sends a batch of SCTs to its integrated auditing component and asks whether the SCTs (and corresponding certificates) have been legitimately added to a log. The auditor can then asynchronously contact the logs and perform the verification.
### VI. Other System Configurations
In addition to the typical configuration described above, where monitors and auditors are tightly integrated with existing TLS/SSL components, Certificate Transparency supports many other configurations. For example, monitors could operate as standalone entities, providing paid or unpaid services to certificate authorities and server operators (see figure 4). A monitor could also be run by a server operator, such as a large Internet entity like Google, Microsoft, or Yahoo. Likewise, an auditor could operate as a standalone service or it could be a secondary function of a monitor.
## Transparent Logs for Skeptical Clients
### I. Cryptographic Hashes, Authentication, and Commitments
A single hash can be an authentication or commitment of an arbitrarily large amount of data, but verification then requires hashing the entire data set. To allow selective verification of subsets of the data, we can use not just a single hash but instead a balanced binary tree of hashes, known as a Merkle tree.
### II. Merkle Trees
1. each node in the tree is hashed
### III. A Merkle Tree-Structured Log
#### A. Storing a Log
Storing the log requires only a few append-only files. The first file holds the log record data, concatenated. The second file is an index of the first, holding a sequence of int64 values giving the start offset of each record in the first file. This index allows efficient random access to any record by its record number. While we could recompute any hash tree from the record data alone, doing so would require N–1 hash operations for a tree of size N. Efficient generation of proofs therefore requires precomputing and storing the hash trees in some more accessible form.
#### B. Serving a Log
Remember that each client consuming the log is skeptical about the log’s correct operation. The log server must make it easy for the client to verify two things
1. that any particular record is in the log
2. that the current log is an append-only extension of a previously-observed earlier log.\
To do all this, the log server must answer five queries:
1. Latest() returns the current log size and top-level hash, cryptographically signed by the server for non-repudiation.
2. RecordProof(R, N) returns the proof that record R is contained in the tree of size N.
3. TreeProof(N, N′) returns the proof that the tree of size N is a prefix of the tree of size N′.
4. Lookup(K) returns the record index R matching lookup key K, if any.
5. Data(R) returns the data associated with record R.
#### C. Verifying a Log
```
validate(bits B as record R):
    if R ≥ cached.N:
        N, T = server.Latest()
        if server.TreeProof(cached.N, N) cannot be verified:
            fail loudly
        cached.N, cached.T = N, T
    if server.RecordProof(R, cached.N) cannot be verified using B:
        fail loudly
    accept B as record R
```
### IV. Summary

