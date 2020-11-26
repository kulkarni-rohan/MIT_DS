# Paper: The Design of a Practical System for Fault-Tolerant Virtual Machines
## Introduction
### I. two approach
1. primary/backup approach: replicating all CPU, memory, I/O devices state -> high bandwidth consuming
2. state-machine approach: same initial state, same input requests in the same order. Extra coordination to keep machines in sync 
### II. Fault Tolerance Protocol
1. based on deterministic replay
2. support only uni-processor VMs
3. only deal with fail-stop failures
## Basic FT Design
primary and backup share the same disk, virtual lockstep, use logging channel. Detect failure by heartbeating and monitoring traffic. Ensure only one is alive.
### I. Deterministic Replay Implementation
#### A. Three Challanges
1. capturing all the input and non-determinism necessary to ensure deterministic execution of a backup virtual machine.
2. apply the inputs and non-determinism to the backup virtual machine
3. don't degrade performance
#### B. Deterministic Replay
1. log file
2. efficient event recording and event delivery
3. no epochs because it's already efficient enough
### II. FT Protocol
#### A. Output Requirement
if primary fails and backup takes over. The backup will execute in a way that is entirely consistent with all outputs that the primary VM has sent to the external world.
#### B. Output Rule
The primary may not send an output to the external world, until the backup VM has received and acknowledged the log entry associated with the operation producing the output. The primary is not stopped to wait.
#### C. Note
we cannot guarantee that all outputs are produced exactly once in a failover situation. Backup cannot determine if a primary crashed before or after sending its last output. We rely on TCP to deal with lost packets and identical (duplicate) packets
### III. Detecting and Responding to Failure
1. UDP heartbeating
2. halt of flow of log entries
3. shared storage test-and-set to avoid split-brain (explained in FAQ)
## Practical Implementation of FT
### I. Starting and Restarting FT VMs
1. VMotion to setup the initial state of backup same as primary
2. backup server is determined by clustering service
### II. Managing the Logging Channel
1. Logging channel is a large buffer for logging entries.
2. primary is producer, backup is consumer
3. gradually slow down primary if the backup lag is too high, if backup catchup, increase the CPU limit gradually.
### III. Operation on FT VMs
1. Various control operations (power off) should be applied to both primary and backup
2. VMotion is independent. To VMotion a primary, backup should switch to the dst VM. To VMotion a backup, primary should temporarily quiesce all of its IOs, so backup don't have IO either
### IV. Implementation Issues for Disk IOs
1. simultaneus disk operations that access the same disk location can lead to non-determinism.
2. a disk operation can also race with a memory access \
bounce buffer: temporary buffer has the same size as the memory being accessed by a disk operation. Data is copied to guest memory only as IO completion is delivered.
3. disk IOs are outstanding on the primary failure: re-issue them after go-live.
### V. Implementation Issues for Network IO
1. disable the asynchronous network optimizations
2. instead, trap to the hypervisor, where it can log the updates and then apply them to the VM.
3. two optimizations:a. batch b. reduce the delay of transmit packets by deferred-execution context.
## Design Alternatives
### I. Shared vs Non-Shared Disk
1. seperate virtual disks, do not have delay according to the Output Rule
2. must be explicitly synced up, and resynced when backup restarted
3. use third party server for tiebreaker to solve split-brain
### II. Executing Disk Reads on the Backup VM
1. greatly reduce the traffic on the logging channel
2. slow down the backup VM's execution
3. primary can't execute second write until backup finished reading the first
4. failed disk read on backup: must wait until it success
5. failed disk read on primary: send to backup through logging channel to ensure same memory 
