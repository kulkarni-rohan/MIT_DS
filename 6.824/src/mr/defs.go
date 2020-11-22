package mr

// task status
const IDLE = 0
const IN_PROGRESS = 1
const COMPLETED = 2

// task type
const TASK_MAP = 0
const TASK_REDUCE = 1

// worker will be killed when not 
// seeing for this amount of time
const WORKER_TIMEOUT = 8

// interval between each master killer check
const KILLER_PERIOD = 2

// interval between each worker send imalive
const IMALIVE_PERIOD = 2