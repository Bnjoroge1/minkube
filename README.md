## Minkube Design Concept

### Manager
manager selects a worker to send tasks to based on a round robin strategy. it then sends a http request to the worker api to perfom the task and gets back a response. 
but the worker might fail because of various reasns such as : a) failure to provide docker image, bug in docker container making it to fail to start, lack of disk space on worker node. 
### Worker
We store the worker state in memory. The task ids are stored in a map data structure. This ensures fault tolerance because if the manager fails, the worker still has information about the tass it should perfom. 
Also, faster because the worker does not need to make a round trip to the manager to get the task ids. It can just read from the map data structure. Periodically, the worker will reconcile its task database with the manager's task database.

TODO: build a distributed key value store for tasks. This will be shared between the manager and the worker. 

For measing CPU utilization, initially used the Perfomance monitoring counters but unfortunately it was not available in my cloudvm. So switched to using the /proc/stat file. 

### Task


