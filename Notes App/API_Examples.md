# API Examples

Complete examples for using the Notes Sync Backend API.

## Table of Contents
- [HTTP REST API](#http-rest-api)
- [WebSocket API](#websocket-api)
- [Client Libraries](#client-libraries)
- [Common Patterns](#common-patterns)

## HTTP REST API

### Create Note

**Request:**
```bash
curl -X POST http://localhost:8080/api/notes/create \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Meeting Notes",
    "content": "Discuss Q4 roadmap",
    "client_id": "user-123"
  }'
```

**Response:**
```json
{
  "id": "note-abc123",
  "title": "Meeting Notes",
  "content": "Discuss Q4 roadmap",
  "crdt": {
    "operations": [],
    "text": "Discuss Q4 roadmap",
    "clock": {
      "user-123": 1
    }
  },
  "clock": {
    "user-123": 1
  },
  "created_at": "2025-02-01T12:00:00Z",
  "updated_at": "2025-02-01T12:00:00Z"
}
```

### Get All Notes

**Request:**
```bash
curl http://localhost:8080/api/notes
```

**Response:**
```json
[
  {
    "id": "note-abc123",
    "title": "Meeting Notes",
    "content": "Discuss Q4 roadmap",
    "crdt": { ... },
    "clock": { "user-123": 1 },
    "created_at": "2025-02-01T12:00:00Z",
    "updated_at": "2025-02-01T12:00:00Z"
  }
]
```

### Sync Notes

**Request:**
```bash
curl -X POST http://localhost:8080/api/sync \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "user-123",
    "notes": [
      {
        "id": "note-abc123",
        "title": "Meeting Notes",
        "content": "Discuss Q4 roadmap and hiring",
        "crdt": {
          "operations": [
            {
              "id": "op-1",
              "client_id": "user-123",
              "clock": { "user-123": 2 },
              "type": "insert",
              "position": 20,
              "content": " and hiring",
              "timestamp": "2025-02-01T12:05:00Z"
            }
          ],
          "text": "Discuss Q4 roadmap and hiring",
          "clock": { "user-123": 2 }
        },
        "clock": { "user-123": 2 }
      }
    ],
    "last_sync": {
      "note-abc123": { "user-123": 1 }
    }
  }'
```

**Response:**
```json
{
  "notes": [
    {
      "id": "note-abc123",
      "title": "Meeting Notes",
      "content": "Discuss Q4 roadmap and hiring plan",
      "crdt": { ... },
      "clock": {
        "user-123": 2,
        "user-456": 1
      }
    }
  ],
  "conflicts": [
    {
      "note_id": "note-abc123",
      "client_clock": { "user-123": 2 },
      "server_clock": { "user-456": 1 },
      "resolution": "merged"
    }
  ],
  "clock": {
    "note-abc123": {
      "user-123": 2,
      "user-456": 1
    }
  }
}
```

## WebSocket API

### Connect

```javascript
const ws = new WebSocket('ws://localhost:8080/ws?client_id=user-123');

ws.onopen = () => {
  console.log('Connected!');
};

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  console.log('Received:', message);
};

ws.onerror = (error) => {
  console.error('Error:', error);
};

ws.onclose = () => {
  console.log('Disconnected');
};
```

### Send Edit Operation

```javascript
const editMessage = {
  type: 'edit',
  payload: {
    note_id: 'note-abc123',
    client_id: 'user-123',
    operation: {
      id: 'op-xyz789',
      client_id: 'user-123',
      clock: { 'user-123': 3 },
      type: 'insert',
      position: 5,
      content: 'Hello ',
      timestamp: new Date().toISOString()
    }
  }
};

ws.send(JSON.stringify(editMessage));
```

### Send Sync Request

```javascript
const syncMessage = {
  type: 'sync',
  payload: {
    client_id: 'user-123',
    notes: [ /* array of notes */ ],
    last_sync: { /* note_id -> clock mapping */ }
  }
};

ws.send(JSON.stringify(syncMessage));
```

### Receive Messages

```javascript
ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  
  switch (message.type) {
    case 'edit':
      // Another user edited a note
      const edit = message.payload;
      console.log('Edit received:', edit);
      applyRemoteEdit(edit);
      break;
      
    case 'sync_response':
      // Response to sync request
      const response = message.payload;
      console.log('Sync response:', response);
      handleSyncResponse(response);
      break;
      
    default:
      console.warn('Unknown message type:', message.type);
  }
};
```

## Client Libraries

### JavaScript/TypeScript

See [examples/client.js](../examples/client.js) for full implementation.

**Basic Usage:**
```javascript
import NotesSyncClient from './client.js';

// Initialize
const client = new NotesSyncClient('http://localhost:8080', 'my-client-id');

// Load cached data
client.loadFromLocalStorage();

// Set up event handlers
client.onNoteUpdate = (note) => {
  console.log('Note updated:', note);
};

client.onConflict = (conflicts) => {
  console.log('Conflicts resolved:', conflicts);
};

client.onConnectionChange = (isOnline) => {
  console.log('Status:', isOnline ? 'Online' : 'Offline');
};

// Connect
await client.connect();

// Create note
const note = await client.createNote('Title', 'Content');

// Edit note
client.editNote(note.id, 0, 'Prefix ');
```

### Python

```python
import requests
import json
import websocket
from threading import Thread

class NotesSyncClient:
    def __init__(self, base_url, client_id):
        self.base_url = base_url
        self.client_id = client_id
        self.ws = None
        
    def create_note(self, title, content):
        response = requests.post(
            f"{self.base_url}/api/notes/create",
            json={
                "title": title,
                "content": content,
                "client_id": self.client_id
            }
        )
        return response.json()
    
    def get_notes(self):
        response = requests.get(f"{self.base_url}/api/notes")
        return response.json()
    
    def sync(self, notes, last_sync):
        response = requests.post(
            f"{self.base_url}/api/sync",
            json={
                "client_id": self.client_id,
                "notes": notes,
                "last_sync": last_sync
            }
        )
        return response.json()
    
    def connect_websocket(self):
        ws_url = self.base_url.replace('http', 'ws')
        self.ws = websocket.WebSocketApp(
            f"{ws_url}/ws?client_id={self.client_id}",
            on_message=self.on_message,
            on_error=self.on_error,
            on_close=self.on_close
        )
        
        wst = Thread(target=self.ws.run_forever)
        wst.daemon = True
        wst.start()
    
    def on_message(self, ws, message):
        data = json.loads(message)
        print(f"Received: {data}")
    
    def on_error(self, ws, error):
        print(f"Error: {error}")
    
    def on_close(self, ws, close_status_code, close_msg):
        print("Connection closed")

# Usage
client = NotesSyncClient('http://localhost:8080', 'python-client')
note = client.create_note('Test', 'Content')
print(note)
```

### Go

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "github.com/gorilla/websocket"
)

type Client struct {
    BaseURL  string
    ClientID string
}

func (c *Client) CreateNote(title, content string) (map[string]interface{}, error) {
    payload := map[string]string{
        "title":     title,
        "content":   content,
        "client_id": c.ClientID,
    }
    
    body, _ := json.Marshal(payload)
    resp, err := http.Post(
        c.BaseURL+"/api/notes/create",
        "application/json",
        bytes.NewBuffer(body),
    )
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    return result, nil
}

func (c *Client) ConnectWebSocket() (*websocket.Conn, error) {
    url := fmt.Sprintf("%s/ws?client_id=%s", c.BaseURL, c.ClientID)
    url = strings.Replace(url, "http", "ws", 1)
    
    conn, _, err := websocket.DefaultDialer.Dial(url, nil)
    return conn, err
}

func main() {
    client := &Client{
        BaseURL:  "http://localhost:8080",
        ClientID: "go-client",
    }
    
    note, _ := client.CreateNote("Test", "Content")
    fmt.Println(note)
}
```

## Common Patterns

### 1. Offline-First Architecture

```javascript
class OfflineFirstApp {
  constructor(serverUrl) {
    this.client = new NotesSyncClient(serverUrl);
    this.syncInterval = null;
  }
  
  async init() {
    // Load from local storage immediately
    this.client.loadFromLocalStorage();
    this.renderUI();
    
    // Try to connect in background
    this.attemptConnection();
    
    // Set up periodic sync
    this.syncInterval = setInterval(() => {
      if (this.client.isOnline) {
        this.client.sync();
      }
    }, 30000); // Sync every 30 seconds
  }
  
  async attemptConnection() {
    try {
      await this.client.connect();
      this.showOnlineIndicator();
    } catch (error) {
      // Work offline, will retry
      this.showOfflineIndicator();
      setTimeout(() => this.attemptConnection(), 5000);
    }
  }
  
  createNote(title, content) {
    // Always works, even offline
    const note = this.client.createNote(title, content);
    this.renderNote(note);
    return note;
  }
  
  editNote(noteId, position, content) {
    // Instant local update
    this.client.editNote(noteId, position, content);
    // Syncs automatically when online
  }
}
```

### 2. Real-Time Collaboration

```javascript
class CollaborativeEditor {
  constructor(noteId, serverUrl, userId) {
    this.noteId = noteId;
    this.client = new NotesSyncClient(serverUrl, userId);
    this.cursors = new Map(); // Track other users' cursors
  }
  
  async init() {
    await this.client.connect();
    
    // Listen for remote edits
    this.client.onNoteUpdate = (note) => {
      if (note.id === this.noteId) {
        this.updateEditor(note.content);
      }
    };
    
    // Set up editor change handler
    this.editor.on('change', (position, content, isDelete) => {
      this.client.editNote(this.noteId, position, content, isDelete);
    });
  }
  
  updateEditor(content) {
    // Update editor without triggering change event
    const cursorPos = this.editor.getCursorPosition();
    this.editor.setValue(content);
    this.editor.setCursorPosition(cursorPos);
  }
  
  showUserCursor(userId, position) {
    this.cursors.set(userId, position);
    this.renderCursors();
  }
}
```

### 3. Conflict Resolution UI

```javascript
class ConflictHandler {
  constructor(client) {
    this.client = client;
    
    this.client.onConflict = (conflicts) => {
      this.handleConflicts(conflicts);
    };
  }
  
  handleConflicts(conflicts) {
    for (const conflict of conflicts) {
      // Show notification
      this.showNotification(
        `Merged changes in "${conflict.note_id}"`,
        'info'
      );
      
      // Highlight merged note
      this.highlightNote(conflict.note_id);
      
      // Log for debugging
      console.log('Conflict resolved:', {
        noteId: conflict.note_id,
        resolution: conflict.resolution,
        clientClock: conflict.client_clock,
        serverClock: conflict.server_clock
      });
    }
  }
  
  showNotification(message, type) {
    // Show toast/notification to user
    const notification = document.createElement('div');
    notification.className = `notification ${type}`;
    notification.textContent = message;
    document.body.appendChild(notification);
    
    setTimeout(() => notification.remove(), 3000);
  }
  
  highlightNote(noteId) {
    const element = document.getElementById(`note-${noteId}`);
    if (element) {
      element.classList.add('highlight');
      setTimeout(() => element.classList.remove('highlight'), 2000);
    }
  }
}
```

### 4. Optimistic Updates

```javascript
class OptimisticUI {
  constructor(client) {
    this.client = client;
    this.pendingOperations = new Map();
  }
  
  async createNote(title, content) {
    // Generate temporary ID
    const tempId = `temp-${Date.now()}`;
    
    // Show immediately in UI
    this.renderNote({ id: tempId, title, content, pending: true });
    
    try {
      // Create on server
      const note = await this.client.createNote(title, content);
      
      // Replace temp note with real note
      this.replaceNote(tempId, note);
    } catch (error) {
      // Show error, keep temp note for retry
      this.showError('Failed to create note. Will retry...');
      this.pendingOperations.set(tempId, { title, content });
    }
  }
  
  async retryPending() {
    for (const [tempId, { title, content }] of this.pendingOperations) {
      try {
        const note = await this.client.createNote(title, content);
        this.replaceNote(tempId, note);
        this.pendingOperations.delete(tempId);
      } catch (error) {
        // Will retry later
      }
    }
  }
}
```

### 5. Auto-Save

```javascript
class AutoSave {
  constructor(client, noteId) {
    this.client = client;
    this.noteId = noteId;
    this.saveTimeout = null;
    this.lastSave = Date.now();
  }
  
  handleEdit(position, content, isDelete) {
    // Clear previous timeout
    clearTimeout(this.saveTimeout);
    
    // Apply edit immediately
    this.client.editNote(this.noteId, position, content, isDelete);
    
    // Show saving indicator
    this.showSavingIndicator();
    
    // Debounce sync
    this.saveTimeout = setTimeout(() => {
      this.save();
    }, 1000); // Save after 1 second of no edits
  }
  
  async save() {
    try {
      await this.client.sync();
      this.lastSave = Date.now();
      this.showSavedIndicator();
    } catch (error) {
      this.showErrorIndicator();
    }
  }
  
  showSavingIndicator() {
    document.getElementById('status').textContent = 'Saving...';
  }
  
  showSavedIndicator() {
    const status = document.getElementById('status');
    status.textContent = 'Saved';
    setTimeout(() => status.textContent = '', 2000);
  }
}
```

## Error Handling

### Network Errors

```javascript
async function robustSync(client) {
  let retries = 3;
  let delay = 1000;
  
  while (retries > 0) {
    try {
      await client.sync();
      return;
    } catch (error) {
      retries--;
      
      if (retries === 0) {
        console.error('Sync failed after retries:', error);
        throw error;
      }
      
      console.log(`Retry in ${delay}ms...`);
      await new Promise(resolve => setTimeout(resolve, delay));
      delay *= 2; // Exponential backoff
    }
  }
}
```

### Validation

```javascript
function validateNote(note) {
  if (!note.id || typeof note.id !== 'string') {
    throw new Error('Invalid note ID');
  }
  
  if (!note.title || note.title.length > 200) {
    throw new Error('Invalid note title');
  }
  
  if (note.content && note.content.length > 1000000) {
    throw new Error('Note content too large');
  }
  
  return true;
}
```

## Performance Tips

1. **Batch Operations**: Group multiple edits into single sync
2. **Debounce Saves**: Don't sync on every keystroke
3. **Local First**: Always update UI immediately
4. **Lazy Loading**: Load notes on-demand for large collections
5. **Compression**: Compress large operation logs before sending

## Security Best Practices

1. **Validate Input**: Always validate user input
2. **Rate Limiting**: Implement client-side rate limiting
3. **Authentication**: Add token-based auth
4. **HTTPS/WSS**: Use secure connections in production
5. **Sanitize Output**: Escape user content in UI
