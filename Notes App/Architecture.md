# Architecture Documentation

## System Overview

The Notes Sync Backend is an offline-first synchronization system that ensures conflict-free collaboration on text documents. It uses CRDTs (Conflict-free Replicated Data Types) and vector clocks to provide automatic conflict resolution without user intervention.

## Core Principles

### 1. Offline-First
- **Local-First**: All changes happen locally first
- **Eventual Consistency**: Changes sync when connection is available
- **Optimistic Updates**: Never block the user waiting for server
- **Queue-Based**: Operations queued when offline, synced when online

### 2. Conflict-Free
- **Deterministic Resolution**: Same inputs always produce same output
- **Commutative Operations**: Order of operations doesn't matter
- **Associative Merging**: Merging is transitive and consistent
- **No Data Loss**: All concurrent edits are preserved

### 3. Real-Time
- **WebSocket Push**: Server pushes updates to connected clients
- **Operation Streaming**: Edits streamed as they happen
- **Low Latency**: Sub-100ms update propagation
- **Automatic Reconnection**: Resilient to network issues

## Component Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Client Layer                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Browser    │  │   Mobile     │  │   Desktop    │      │
│  │   Client     │  │   Client     │  │   Client     │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
│         │                  │                  │              │
│         └──────────────────┴──────────────────┘              │
│                            │                                 │
└────────────────────────────┼─────────────────────────────────┘
                             │
                   ┌─────────▼─────────┐
                   │   HTTP / WS API   │
                   └─────────┬─────────┘
                             │
┌────────────────────────────┼─────────────────────────────────┐
│                    Backend Layer                              │
│                            │                                  │
│   ┌────────────────────────▼──────────────────────┐          │
│   │           API Server (Gorilla)                 │          │
│   │  ┌──────────────┐    ┌──────────────┐        │          │
│   │  │  HTTP        │    │  WebSocket   │        │          │
│   │  │  Handlers    │    │  Manager     │        │          │
│   │  └──────┬───────┘    └──────┬───────┘        │          │
│   └─────────┼───────────────────┼─────────────────┘          │
│             │                   │                             │
│   ┌─────────▼───────────────────▼─────────────┐              │
│   │         Sync Engine                       │              │
│   │  ┌──────────────┐  ┌──────────────┐      │              │
│   │  │   CRDT       │  │   Vector     │      │              │
│   │  │   Merger     │  │   Clock      │      │              │
│   │  └──────────────┘  └──────────────┘      │              │
│   │  ┌──────────────────────────────┐        │              │
│   │  │   Conflict Resolution        │        │              │
│   │  └──────────────────────────────┘        │              │
│   └─────────────────┬─────────────────────────┘              │
│                     │                                         │
│   ┌─────────────────▼─────────────────────┐                  │
│   │        Storage Layer (SQLite)         │                  │
│   │  ┌──────────────┐  ┌──────────────┐  │                  │
│   │  │    Notes     │  │  Operations  │  │                  │
│   │  │    Table     │  │    Log       │  │                  │
│   │  └──────────────┘  └──────────────┘  │                  │
│   └───────────────────────────────────────┘                  │
│                                                               │
└───────────────────────────────────────────────────────────────┘
```

## Data Structures

### 1. Vector Clock
```go
type VectorClock map[string]int64
```

**Purpose**: Track causality between operations across distributed clients.

**Properties**:
- Each client has its own counter
- Incremented on local operations
- Merged by taking max of each component
- Enables happens-before relationship detection

**Example**:
```go
// Client A's clock after 3 operations
clockA = {"client-A": 3, "client-B": 0}

// Client B's clock after 2 operations
clockB = {"client-A": 0, "client-B": 2}

// After sync, both have
merged = {"client-A": 3, "client-B": 2}
```

### 2. CRDT (Operation-based)
```go
type CRDT struct {
    Operations []Operation
    Text       string
    Clock      VectorClock
}
```

**Purpose**: Represent document state as a sequence of operations.

**Properties**:
- Operations are commutative (order-independent)
- Operations are idempotent (safe to apply multiple times)
- All replicas converge to same state
- Supports insert and delete operations

**Operation Types**:
```go
type Operation struct {
    ID       string        // Unique operation ID
    ClientID string        // Client that created it
    Clock    VectorClock   // Causal timestamp
    Type     string        // "insert" or "delete"
    Position int           // Character position
    Content  string        // Content to insert
}
```

### 3. Note
```go
type Note struct {
    ID        string
    Title     string
    Content   string        // Current text state
    CRDT      *crdt.CRDT    // Operation log
    Clock     VectorClock   // Current version
    CreatedAt time.Time
    UpdatedAt time.Time
    DeletedAt *time.Time    // Soft delete
}
```

## Algorithms

### 1. Conflict Detection

```go
func detectConflict(clientClock, serverClock VectorClock) ConflictType {
    cmp := clientClock.Compare(serverClock)
    
    switch cmp {
    case -1:  // client < server
        return ServerWins
    case 1:   // client > server
        return ClientWins
    case 0:   // concurrent
        return NeedsMerge
    }
}
```

**Cases**:
- **ServerWins**: Client is behind, discard client changes
- **ClientWins**: Server is behind, accept client changes
- **NeedsMerge**: Concurrent edits, use CRDT to merge

### 2. CRDT Merge

```go
func (c *CRDT) Merge(other *CRDT) error {
    // Get operations we don't have
    newOps := findMissingOperations(c.Operations, other.Operations)
    
    // Apply all new operations
    for _, op := range newOps {
        c.ApplyOperation(op)
    }
    
    // Merge vector clocks
    c.Clock.Update(other.Clock)
    
    // Rebuild text from all operations
    c.rebuildText()
    
    return nil
}
```

**Process**:
1. Identify missing operations
2. Apply them to local CRDT
3. Update vector clock
4. Reconstruct text from operations

### 3. Text Reconstruction

```go
func (c *CRDT) rebuildText() {
    // Sort operations by causal order
    sorted := sortByCausality(c.Operations)
    
    text := []rune{}
    for _, op := range sorted {
        switch op.Type {
        case "insert":
            text = insert(text, op.Position, op.Content)
        case "delete":
            text = delete(text, op.Position)
        }
    }
    
    c.Text = string(text)
}
```

**Ordering**:
1. Primary: Vector clock comparison
2. Secondary: Timestamp (for concurrent ops)
3. Tertiary: Client ID (deterministic tiebreaker)

## Synchronization Protocol

### HTTP Sync Flow

```
Client                                Server
  |                                     |
  |------- POST /api/sync ------------→ |
  |  {                                  |
  |    client_id: "abc",                |
  |    notes: [note1, note2],           |
  |    last_sync: {                     |
  |      "note1": {client: 5}           |
  |    }                                |
  |  }                                  |
  |                                     |
  |                                     | 1. Compare clocks
  |                                     | 2. Detect conflicts
  |                                     | 3. Merge CRDTs
  |                                     | 4. Build response
  |                                     |
  | ←------------ Response -------------|
  |  {                                  |
  |    notes: [updated_notes],          |
  |    conflicts: [{                    |
  |      note_id: "note1",              |
  |      resolution: "merged"           |
  |    }],                              |
  |    clock: {                         |
  |      "note1": {client: 5, other: 3} |
  |    }                                |
  |  }                                  |
  |                                     |
```

### WebSocket Real-Time Flow

```
Client A          Server          Client B
  |                 |                 |
  |--- connect ----→|←---- connect ---|
  |                 |                 |
  |--- edit -------→|                 |
  |  {              |                 |
  |    type: "edit",|                 |
  |    note_id: "1",|                 |
  |    operation: {…}                 |
  |  }              |                 |
  |                 |                 |
  |                 | 1. Apply op     |
  |                 | 2. Save to DB   |
  |                 | 3. Broadcast    |
  |                 |                 |
  |                 |--- edit -------→|
  |                 |                 | 4. Apply locally
  |                 |                 | 5. Update UI
```

## Database Schema

### Notes Table
```sql
CREATE TABLE notes (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    crdt_data TEXT NOT NULL,      -- JSON serialized CRDT
    clock_data TEXT NOT NULL,     -- JSON serialized vector clock
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME           -- Soft delete
);

CREATE INDEX idx_notes_updated_at ON notes(updated_at);
CREATE INDEX idx_notes_deleted_at ON notes(deleted_at);
```

### Operations Table
```sql
CREATE TABLE operations (
    id TEXT PRIMARY KEY,
    note_id TEXT NOT NULL,
    client_id TEXT NOT NULL,
    operation_data TEXT NOT NULL,  -- JSON serialized operation
    clock_data TEXT NOT NULL,      -- JSON serialized vector clock
    timestamp DATETIME NOT NULL,
    FOREIGN KEY (note_id) REFERENCES notes(id)
);

CREATE INDEX idx_operations_note_id ON operations(note_id);
CREATE INDEX idx_operations_timestamp ON operations(timestamp);
```

## Performance Characteristics

### Time Complexity
- **Insert/Delete Operation**: O(1) to create
- **Text Rebuild**: O(n log n) where n = number of operations
- **CRDT Merge**: O(m) where m = new operations
- **Conflict Detection**: O(k) where k = number of clients

### Space Complexity
- **Per Note**: O(n) where n = total operations
- **Vector Clock**: O(k) where k = number of clients
- **In-Memory**: O(c × n) where c = connected clients

### Scalability Considerations

**Current Design** (Single Server):
- Handles: ~10,000 concurrent connections
- Throughput: ~1,000 operations/second
- Latency: <50ms p99

**Scaling Strategies**:
1. **Horizontal**: Redis pub/sub for cross-server sync
2. **Database**: PostgreSQL for better concurrent writes
3. **Caching**: Redis for hot notes
4. **Sharding**: Partition notes by ID hash

## Error Handling

### Network Failures
- Client queues operations locally
- Retry with exponential backoff
- Resume sync when reconnected

### Concurrent Modifications
- CRDT automatically merges
- No rollbacks needed
- All edits preserved

### Data Corruption
- Operations are immutable
- Can rebuild state from log
- Regular integrity checks

## Security Considerations

### Authentication
```go
// Add to each request
type AuthHeader struct {
    ClientID string
    Token    string  // JWT token
}
```

### Authorization
```go
// Check note ownership
func (s *Store) CheckAccess(noteID, userID string) bool {
    // Query permissions table
}
```

### Rate Limiting
```go
// Per-client rate limits
type RateLimit struct {
    Operations int  // 100 ops/minute
    Syncs      int  // 10 syncs/minute
}
```

## Monitoring & Observability

### Metrics to Track
```go
// Prometheus metrics
var (
    syncCount = prometheus.NewCounter(...)
    conflictCount = prometheus.NewCounter(...)
    operationLatency = prometheus.NewHistogram(...)
    activeConnections = prometheus.NewGauge(...)
)
```

### Logging Strategy
```go
// Structured logging
log.WithFields(log.Fields{
    "note_id": noteID,
    "client_id": clientID,
    "operation": "merge",
    "conflict": true,
}).Info("CRDT merge completed")
```

### Health Checks
- Database connectivity
- WebSocket connection pool
- Operation queue depth
- Memory usage

## Testing Strategy

### Unit Tests
- Vector clock operations
- CRDT insert/delete
- Conflict detection
- Text reconstruction

### Integration Tests
- End-to-end sync flow
- WebSocket messaging
- Database persistence
- Concurrent client simulation

### Stress Tests
- 1000+ concurrent clients
- Rapid operation bursts
- Network partition scenarios
- Database failure recovery

## Future Enhancements

### Planned Features
1. **Rich Text Support**: Extend CRDT for formatting
2. **Attachments**: File sync with content addressing
3. **Permissions**: Fine-grained access control
4. **History**: Time-travel through operation log
5. **Compression**: Compact operation log over time

### Performance Improvements
1. **Operation Compaction**: Merge old operations
2. **Lazy Loading**: Stream operations on demand
3. **Delta Sync**: Send only diffs, not full state
4. **Parallel Processing**: Concurrent CRDT merges
