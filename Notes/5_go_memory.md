# The Go Memory Model
## I. Happens Before
1. if a happends before b, then b happends after a.
2. if a does not happen before b, b does not happen before a, then a and b happen concurrently
## II. Channel Communication
The kth receive on a channel with capacity C happends before the k+Cth send from that channel completes\
in human words: sender wait until channel is not full. reader wait until channel is not empty.
## III. Locks
1. for any sync.Mutex or sync.RWMutex variable l and n < m, call n of l.Unlock() happens before call m of l.Lock() returns \
in human words: it won't unlock until it's locked; it won't lock until it's unlocked
2. for any call to l.RLock on a sync.RWMutex variable l, there is an n such that the l.RLock happens (returns) after call n to l.Unlock and the matching l.RUnlock happens before call n+1 to l.Lock() \
in human words: write lock won't lock until read lock released; read lock won't lock until write lock released.
## IV. Once
single call of f() from once.Do(f) happens (returns) before any call of once.Do(f) returns、
in human words: multiple call to once.Do(f) only result in once execution