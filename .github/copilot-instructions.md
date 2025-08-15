# Minkube AI Coding Agent Instructions

## Project Overview
Minkube is a distributed container orchestration system written in Go with a manager-worker architecture. The manager handles task distribution and coordination while workers execute containerized tasks using Docker.

## Architecture & Core Components

### Manager-Worker Distribution
- **Manager** (`manager/`): HTTP API server, task distribution, worker coordination
- **Worker** (`worker/`): Task execution, container lifecycle management via Docker API
- **Communication**: HTTP-based with optimized connection pooling
- **State Sync**: Manager polls workers every 15 seconds via `updateTasks()`

### Key Data Structures
- `Task`: Core work unit with Docker container specs (`task/task.go`)
- `TaskEvent`: State transition wrapper with UUID, timestamp, state
- `Manager`: Orchestrates with `TaskDb`, `PendingTasks` queue, `WorkersTaskMap`
- States: `Pending → Scheduled → Running → Completed/Failed` (`task/state_machine.go`)

### Critical Performance Optimizations
The system was heavily optimized for memory efficiency and HTTP performance:

#### HTTP Connection Pooling
Manager uses connection pooling to prevent connection creation overhead:
```go
var httpClient = &http.Client{
    Transport: &http.Transport{
        MaxIdleConns: 100,
        MaxIdleConnsPerHost: 10, // Key: reuse connections per worker
        IdleConnTimeout: 90*time.Second,
        DisableKeepAlives: false,
    },
}
```

#### Memory Management Patterns
- **Background cleanup**: `CleanUpTasks()` runs every hour to remove old completed tasks
- **Emergency cleanup**: Triggered when `TaskDb` exceeds threshold
- **Pagination**: All endpoints use `?page=1&limit=100` to prevent memory exhaustion
- **Byte pool reuse**: JSON decoder pooling to avoid reflection metadata accumulation

## Development Workflows

### Multi-Process Debugging Setup
Use VS Code compound configuration "Debug All: Manager + 3 Workers":
```bash
# Via Makefile - preferred method
make run-workers-dev    # Start 3 workers (ports 8001-8003)  
make run-manager-multi  # Manager connecting to all workers
make stop-workers       # Cleanup background workers
```

### Load Testing & Memory Profiling
```bash
# Load test with memory monitoring
hey -n 200000 -c 1000 -m POST -T "application/json" \
    -d '{"task":{"name":"test","image":"nginx","memory":128}}' \
    http://localhost:8080/tasks

# Memory profiling
go tool pprof http://localhost:8080/debug/pprof/heap
```

### Build & Run Targets
- `make run-worker WORKER_PORT=8001` - Single worker on custom port
- `make run-manager-multi` - Manager with distributed workers
- `make build-all` - Cross-platform binaries
- `make debug-manager` / `make debug-worker` - Debug mode with verbose logging

## Project-Specific Conventions

### Request Validation & Error Handling
API handlers validate Docker requirements and use structured errors:
```go
// Memory minimum: 6MB (Docker requirement)
const minMemoryMB = 6
memoryInBytes := taskRequestEvent.Task.Memory * 1024 * 1024
if memoryInBytes < minMemoryBytes {
    e := ErrResponse{HTTPStatusCode: 400, Message: "Memory must be at least 6MB"}
    json.NewEncoder(w).Encode(e)
    return
}
```

### Task Lifecycle Management
1. Client POST → `StartTaskHandler` generates UUID, validates memory/disk
2. Task enters `PendingTasks` queue → `SendWork()` distributes via round-robin
3. Workers execute → Manager polls for updates via `updateTasks()`
4. Background cleanup removes completed tasks after retention period

### Environment Variables Pattern
All processes use consistent env vars:
- `MINKUBE_HOST` - Bind address
- `MINKUBE_PORT` - Listen port  
- `MINKUBE_ROLE` - "manager" or "worker"
- `MINKUBE_WORKERS` - Comma-separated worker addresses for manager

## Integration Points

### Manager-Worker Synchronization
- Worker exposes `GET /tasks?page=1&limit=1000` with pagination metadata
- Manager expects `PaginatedTaskResponse{Tasks: [...], Pagination: {...}}`
- Orphaned task handling: Manager adds worker-reported tasks missing from `TaskDb`
- State reconciliation: Workers report actual container states back to manager

### Docker Integration Patterns
- Workers manage full container lifecycle via Docker API
- Memory specs: API accepts MB, converts to bytes for Docker
- Container IDs populated by workers, never by manager
- Port bindings: Docker auto-assigns, worker reports back to manager

### State Machine Transitions
```go
var stateTransitionMap = map[State][]State{
    Pending:   []State{Scheduled},
    Scheduled: []State{Scheduled, Failed, Running},
    Running:   []State{Running, Completed, Failed},
    Completed: []State{}, // terminal
    Failed:    []State{}, // terminal
}
```

## Common Gotchas & Anti-Patterns

### Memory & Performance
- **Pagination required**: Never return unbounded task lists (causes OOM at scale)
- **Connection reuse**: Short timeouts break HTTP connection pooling benefits
- **JSON reflection**: Avoid creating new decoders in hot paths

### Concurrency & State
- **Lock granularity**: Use `sync.RWMutex` for read-heavy operations like `GetTasks`
- **UUID comparison**: Use `task.ID == uuid.Nil`, not `&task.ID == &uuid.Nil`
- **Shared state**: Always hold appropriate locks when accessing `TaskDb`, `WorkersTaskMap`

### Docker & Container Management
- **Memory units**: API uses MB, Docker requires bytes, validate minimums (6MB)
- **Container inspection**: Handle "No such container" errors gracefully
- **Port mappings**: Docker assigns random ports, don't assume specific values

### HTTP & API Design
- **Status codes**: Use `201 Created` for task creation, `204 No Content` for deletion
- **Request limits**: Set `MaxBytesReader` to prevent large request DOS
- **Content validation**: Always validate required fields (`name`, `image`)

## Production Considerations

### Planned Enhancements (from README.md)
- **Database migration**: Move from in-memory maps to PostgreSQL
- **Service discovery**: Implement DNS-based discovery vs current HTTP registration
- **Authentication**: API keys, JWT tokens, mTLS for worker communication
- **Monitoring**: Prometheus metrics, distributed tracing, structured logging

### Resource Management
- **Background processes**: Manager runs cleanup, task processing, and worker sync in separate goroutines
- **Worker monitoring**: `MonitorTasks()` reconciles container states every 15 seconds
- **Graceful shutdown**: Signal handling for clean process termination

### Debugging & Observability
- **pprof integration**: Both manager and worker expose `/debug/pprof/` endpoints
- **Trace collection**: Runtime tracing enabled via `trace.Start()`
- **Structured logging**: Consistent log formats with task IDs for correlation

This architecture emphasizes fault tolerance, performance optimization, and operational simplicity while maintaining clear separation of concerns between orchestration (manager) and execution (workers).
