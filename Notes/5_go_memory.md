# The Go Memory Model
## I. Happens Before
1. if a happends before b, then b happends after a.
2. if a does not happen before b, b does not happen before a, then a and b happen concurrently
## II. Channel Communication
The kth receive on a channel with capacity C happends before the k+Cth send from that channel completes
## III. Locks
1. for any sync.Mutex or sync.RWMutex variable l and n < m, call n of l.Unlock() happens before call m of l.Lock() returns
2. for any call to l.RLock on a sync.RWMutex variable l, there is an n such that the l.RLock happens (returns) after call n to l.Unlock and the matching l.RUnlock happens before call n+1 to l.Lock()
## IV. Once
single call of f() from once.Do(f) happens (returns) before any call of once.Do(f) returns