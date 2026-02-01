# Quick Start Guide

Get your Notes Sync Backend up and running in minutes!

## Prerequisites

- Go 1.21+ installed
- SQLite3 (usually pre-installed)
- Git

## Installation

### Option 1: Run Directly

```bash
# Clone the repository
git clone <your-repo-url>
cd notes-sync-backend

# Install dependencies
go mod download

# Run the server
go run ./cmd/server/main.go
```

The server will start on `http://localhost:8080`

### Option 2: Build Binary

```bash
# Build
make build

# Run
./notes-sync-server
```

### Option 3: Docker

```bash
# Using docker-compose (recommended)
docker-compose up

# Or build and run manually
docker build -t notes-sync .
docker run -p 8080:8080 -v $(pwd)/data:/data notes-sync
```

## First Steps

### 1. Check Server Health

```bash
curl http://localhost:8080/health
# Response: OK
```

### 2. Open the Demo UI

Visit `http://localhost:8080` in your browser to see the interactive demo.

### 3. Create Your First Note

**Via Demo UI:**
1. Click "Connect WebSocket"
2. Enter a title and content
3. Click "Create Note"

**Via API:**
```bash
curl -X POST http://localhost:8080/api/notes/create \
  -H "Content-Type: application/json" \
  -d '{
    "title": "My First Note",
    "content": "Hello, World!",
    "client_id": "my-client"
  }'
```

### 4. Test Offline Editing

**Browser Console:**
```javascript
// Open developer console
const client = new NotesSyncClient('http://localhost:8080', 'test-client');

// Load from local storage
client.loadFromLocalStorage();

// Connect
await client.connect();

// Create a note
const note = await client.createNote('Test', 'Content');

// Edit while online
client.editNote(note.id, 0, 'Hello ');

// Disconnect to simulate offline
client.ws.close();

// Edit while offline (operations are queued)
client.editNote(note.id, 6, 'World');

// Reconnect (operations sync automatically)
await client.connect();
```

## Testing Conflict Resolution

Open two browser tabs to test concurrent editing:

### Tab 1:
1. Connect to server
2. Create a note: "ABC"
3. Disconnect
4. Edit to: "AXBC" (insert X at position 1)

### Tab 2:
1. Connect to server
2. See the same note: "ABC"
3. Edit to: "AYBC" (insert Y at position 1)

### Tab 1:
1. Reconnect
2. **See result: "AXYBC" or "AYXBC"**
3. Both edits preserved!

## Configuration

Set environment variables before running:

```bash
# Port (default: 8080)
export PORT=3000

# Database path (default: ./notes.db)
export DB_PATH=/var/lib/notes.db

# Run with custom config
go run ./cmd/server/main.go
```

## Running Tests

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run with race detector
make test-race

# View coverage report
open coverage.html
```

## Development Workflow

```bash
# Format code
make fmt

# Run linter
make vet

# Run all checks
make all

# Clean build artifacts
make clean
```

## Common Use Cases

### 1. Building a Note-Taking App

```javascript
// Initialize client
const client = new NotesSyncClient('http://localhost:8080');
client.loadFromLocalStorage();

// Connect when online
window.addEventListener('online', () => client.connect());
window.addEventListener('offline', () => client.ws?.close());

// React to updates
client.onNoteUpdate = (note) => {
  updateUI(note);
};

// Handle user edits
function handleEdit(noteId, position, content) {
  client.editNote(noteId, position, content);
  // UI updates immediately, syncs in background
}
```

### 2. Collaborative Editing

```javascript
// Multiple users editing the same note
const alice = new NotesSyncClient('http://localhost:8080', 'alice');
const bob = new NotesSyncClient('http://localhost:8080', 'bob');

await Promise.all([alice.connect(), bob.connect()]);

// Both edit simultaneously
alice.editNote(noteId, 0, 'Alice: ');
bob.editNote(noteId, 0, 'Bob: ');

// Changes merge automatically
// Final text contains both edits
```

### 3. Offline-First Mobile App

```javascript
// Always work offline-first
class NotesApp {
  async init() {
    this.client = new NotesSyncClient('https://api.example.com');
    this.client.loadFromLocalStorage();
    
    // Try to connect, but don't wait
    this.client.connect().catch(() => {
      // Work offline, will sync later
    });
  }
  
  createNote(title, content) {
    // Works immediately, even offline
    return this.client.createNote(title, content);
  }
  
  editNote(id, pos, text) {
    // Instant response
    this.client.editNote(id, pos, text);
  }
}
```

## Troubleshooting

### Server won't start
```bash
# Check if port is already in use
lsof -i :8080

# Use different port
PORT=3000 go run ./cmd/server/main.go
```

### Database locked
```bash
# Remove existing database
rm notes.db

# Restart server
go run ./cmd/server/main.go
```

### WebSocket connection fails
```bash
# Check firewall settings
# Ensure browser allows WebSocket connections
# Try different browser

# Check server logs for errors
```

### Operations not syncing
```javascript
// Check connection status
console.log('Online:', client.isOnline);
console.log('WebSocket:', client.ws?.readyState);

// Force sync
await client.sync();
```

## API Quick Reference

### REST Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/notes/create` | Create new note |
| GET | `/api/notes` | Get all notes |
| POST | `/api/sync` | Sync notes |
| GET | `/health` | Health check |

### WebSocket

| Message Type | Direction | Description |
|-------------|-----------|-------------|
| `edit` | Client → Server | Apply operation |
| `edit` | Server → Client | Broadcast operation |
| `sync` | Client → Server | Sync request |
| `sync_response` | Server → Client | Sync response |

## Next Steps

- Read [ARCHITECTURE.md](ARCHITECTURE.md) for deep dive
- Check [examples/client.js](examples/client.js) for full client implementation
- Explore the demo UI source in [cmd/server/main.go](cmd/server/main.go)
- Run tests to understand the algorithms

## Getting Help

- Check logs: Server prints detailed logs to stdout
- Enable debug: Set `LOG_LEVEL=debug`
- Review tests: Test files show usage examples
- Open an issue: Describe your problem with logs

## Production Deployment

Before deploying to production:

1. **Add Authentication**
   - Implement JWT token verification
   - Add user session management

2. **Use Production Database**
   - Switch to PostgreSQL
   - Set up replication

3. **Enable TLS**
   - Use wss:// for WebSocket
   - Configure HTTPS

4. **Add Monitoring**
   - Set up logging aggregation
   - Add metrics collection
   - Configure alerts

5. **Scale Horizontally**
   - Use Redis for pub/sub
   - Deploy multiple instances
   - Add load balancer

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed production recommendations.
