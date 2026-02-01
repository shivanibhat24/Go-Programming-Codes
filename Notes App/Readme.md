# Notes Sync Backend

A production-ready offline-first notes synchronization backend built in Go with automatic conflict resolution using CRDTs (Conflict-free Replicated Data Types) and vector clocks.

## 🎯 Features

- **Offline-First**: Clients can edit notes offline and sync when reconnected
- **Conflict-Free**: Automatic conflict resolution using CRDT and vector clocks
- **Real-Time Sync**: WebSocket support for instant updates across clients
- **Never Loses Data**: All operations are logged and can be replayed
- **Feels Instant**: Optimistic updates with eventual consistency
- **REST API**: Full HTTP API for compatibility
- **Persistent Storage**: SQLite database for reliable persistence

## 🏗️ Architecture

### Core Components

1. **Vector Clocks** (`pkg/clock`)
   - Logical timestamps for tracking causality
   - Determines operation ordering across distributed clients

2. **CRDT** (`pkg/crdt`)
   - Conflict-free Replicated Data Type for text
   - Ensures convergence without coordination
   - Operation-based CRDT with insert/delete operations

3. **Sync Engine** (`internal/sync`)
   - Handles synchronization logic
   - Merges concurrent edits
   - Resolves conflicts automatically

4. **Storage** (`internal/store`)
   - SQLite-based persistence
   - Stores notes and operation logs
   - Enables operation replay

5. **API Server** (`internal/api`)
   - HTTP REST endpoints
   - WebSocket for real-time updates
   - Broadcasts edits to connected clients

## 📦 Installation

### Prerequisites

- Go 1.21 or higher
- SQLite3

### Build

```bash
# Clone the repository
git clone <repository-url>
cd notes-sync-backend

# Download dependencies
go mod download

# Build the server
go build -o server ./cmd/server

# Run the server
./server
```

Or run directly:

```bash
go run ./cmd/server/main.go
```

## 🚀 Usage

### Starting the Server

```bash
# Default: port 8080, database at ./notes.db
go run ./cmd/server/main.go

# Custom configuration via environment variables
PORT=3000 DB_PATH=/var/lib/notes.db go run ./cmd/server/main.go
```

### Access the Demo UI

Open your browser to `http://localhost:8080` to see the interactive demo.

## 📡 API Reference

### REST Endpoints

#### Create Note
```http
POST /api/notes/create
Content-Type: application/json

{
  "title": "My Note",
  "content": "Note content",
  "client_id": "client-123"
}
```

#### Get All Notes
```http
GET /api/notes
```

#### Sync Notes
```http
POST /api/sync
Content-Type: application/json

{
  "client_id": "client-123",
  "notes": [
    {
      "id": "note-1",
      "title": "Updated Note",
      "content": "New content",
      "crdt": { ... },
      "clock": { "client-123": 5 }
    }
  ],
  "last_sync": {
    "note-1": { "client-123": 3 }
  }
}
```

**Response:**
```json
{
  "notes": [ ... ],
  "conflicts": [
    {
      "note_id": "note-1",
      "client_clock": { "client-123": 5 },
      "server_clock": { "client-456": 4 },
      "resolution": "merged"
    }
  ],
  "clock": {
    "note-1": { "client-123": 5, "client-456": 4 }
  }
}
```

### WebSocket API

Connect to: `ws://localhost:8080/ws?client_id=YOUR_CLIENT_ID`

#### Message Types

**Edit Operation:**
```json
{
  "type": "edit",
  "payload": {
    "note_id": "note-1",
    "client_id": "client-123",
    "operation": {
      "id": "op-1",
      "client_id": "client-123",
      "clock": { "client-123": 6 },
      "type": "insert",
      "position": 5,
      "content": "Hello",
      "timestamp": "2025-02-01T12:00:00Z"
    }
  }
}
```

**Sync Request:**
```json
{
  "type": "sync",
  "payload": {
    "client_id": "client-123",
    "notes": [ ... ],
    "last_sync": { ... }
  }
}
```

## 🔧 How It Works

### 1. Offline Editing

Clients maintain a local copy of notes with a CRDT structure. All edits create operations that are stored locally:

```go
// Client creates an insert operation
op := crdt.CreateInsertOperation(clientID, position, content)
localCRDT.ApplyOperation(op)
```

### 2. Synchronization

When reconnecting, the client sends:
- All local notes with their CRDT state
- Vector clocks for each note
- Last known sync state

The server:
1. Compares vector clocks to detect conflicts
2. Merges CRDTs for concurrent edits
3. Returns updates the client doesn't have

### 3. Conflict Resolution

**Vector Clock Comparison:**
- If `client_clock < server_clock`: Server wins
- If `client_clock > server_clock`: Client wins  
- If concurrent (neither before the other): **Merge with CRDT**

**CRDT Merging:**
```go
// Both edits are preserved and merged
serverCRDT.Merge(clientCRDT)
finalText := serverCRDT.Text
```

### 4. Real-Time Updates

Connected clients receive instant updates via WebSocket:

```go
// Server broadcasts edits to all clients
server.broadcast <- &WebSocketMessage{
    Type: "edit",
    Payload: editOperation,
}
```

## 🧪 Testing the System

### Test Conflict Resolution

1. Open the demo UI in two browser tabs
2. Connect both via WebSocket
3. Edit the same note in both tabs while one is "offline" (disconnect)
4. Reconnect and observe automatic merging

### Example Scenario

**Client A (offline):**
- Original: "Hello World"
- Edits to: "Hello Beautiful World"

**Client B (online):**
- Original: "Hello World"
- Edits to: "Hello Amazing World"

**After sync:**
- Merged result: "Hello Beautiful Amazing World"
- No data lost, both edits preserved

## 📊 Data Flow

```
┌─────────────┐         ┌──────────────┐         ┌─────────────┐
│  Client A   │────────▶│ Sync Engine  │◀────────│  Client B   │
│  (offline)  │         │              │         │  (online)   │
└─────────────┘         └──────────────┘         └─────────────┘
      │                        │                        │
      │ Edit Operations        │ Store Operations       │
      ▼                        ▼                        ▼
┌─────────────┐         ┌──────────────┐         ┌─────────────┐
│  Local      │         │   SQLite     │         │  Local      │
│  CRDT       │         │   Database   │         │  CRDT       │
└─────────────┘         └──────────────┘         └─────────────┘
      │                        │                        │
      │ Reconnect              │                        │
      └────────────────────────┴────────────────────────┘
                               │
                        ┌──────▼───────┐
                        │   Conflict   │
                        │  Resolution  │
                        │   (CRDT)     │
                        └──────────────┘
```

## 🔒 Data Guarantees

1. **Eventual Consistency**: All replicas converge to the same state
2. **Causality Preservation**: Operations are ordered by happens-before relationship
3. **No Lost Updates**: All operations are logged and can be replayed
4. **Idempotency**: Operations can be applied multiple times safely

## 🚦 Production Considerations

### Scaling

- **Horizontal Scaling**: Use Redis for WebSocket pub/sub across servers
- **Database**: Consider PostgreSQL for multi-server deployments
- **Caching**: Add Redis for frequently accessed notes

### Security

- Add authentication (JWT tokens)
- Validate client permissions per note
- Rate limiting for API endpoints
- TLS for WebSocket connections

### Monitoring

- Add structured logging
- Instrument with metrics (Prometheus)
- Track sync latency and conflict rates
- Monitor database performance

### Optimization

- Implement operation compaction (merge old operations)
- Add garbage collection for deleted notes
- Use connection pooling for database
- Implement client-side caching

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request


## 🙏 Acknowledgments

- CRDT concepts from research papers on operational transformation
- Vector clock implementation based on Lamport's logical clocks
- Inspiration from production systems like Figma and Linear

## 📚 Further Reading

- [CRDTs: Consistency without concurrency control](https://hal.inria.fr/inria-00609399v1/document)
- [Time, Clocks, and the Ordering of Events](https://lamport.azurewebsites.net/pubs/time-clocks.pdf)
- [Conflict-free Replicated Data Types](https://crdt.tech/)
