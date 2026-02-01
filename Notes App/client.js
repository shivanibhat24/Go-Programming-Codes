// Example JavaScript Client for Notes Sync Backend
// This demonstrates how to implement an offline-first client

class NotesSyncClient {
    constructor(serverUrl, clientId) {
        this.serverUrl = serverUrl;
        this.clientId = clientId || this.generateClientId();
        this.ws = null;
        this.notes = new Map(); // Local notes cache
        this.lastSync = new Map(); // Track last sync state per note
        this.pendingOperations = []; // Operations while offline
        this.isOnline = false;
        
        // Event handlers
        this.onNoteUpdate = null;
        this.onConflict = null;
        this.onConnectionChange = null;
    }

    generateClientId() {
        return 'client-' + Math.random().toString(36).substr(2, 9);
    }

    // Connect to the server
    async connect() {
        return new Promise((resolve, reject) => {
            this.ws = new WebSocket(`${this.serverUrl}/ws?client_id=${this.clientId}`);
            
            this.ws.onopen = () => {
                console.log('Connected to server');
                this.isOnline = true;
                if (this.onConnectionChange) this.onConnectionChange(true);
                
                // Sync on connect
                this.sync().then(resolve);
            };
            
            this.ws.onclose = () => {
                console.log('Disconnected from server');
                this.isOnline = false;
                if (this.onConnectionChange) this.onConnectionChange(false);
            };
            
            this.ws.onerror = (error) => {
                console.error('WebSocket error:', error);
                reject(error);
            };
            
            this.ws.onmessage = (event) => {
                const msg = JSON.parse(event.data);
                this.handleMessage(msg);
            };
        });
    }

    // Handle incoming messages
    handleMessage(msg) {
        switch (msg.type) {
            case 'edit':
                this.applyRemoteEdit(msg.payload);
                break;
            case 'sync_response':
                this.handleSyncResponse(msg.payload);
                break;
            default:
                console.warn('Unknown message type:', msg.type);
        }
    }

    // Create a new note
    async createNote(title, content) {
        const note = {
            id: this.generateNoteId(),
            title,
            content,
            crdt: this.createCRDT(content),
            clock: { [this.clientId]: 1 },
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString()
        };
        
        this.notes.set(note.id, note);
        
        // Save locally
        this.saveToLocalStorage();
        
        // If online, send to server
        if (this.isOnline) {
            try {
                const response = await fetch(`${this.serverUrl}/api/notes/create`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        title,
                        content,
                        client_id: this.clientId
                    })
                });
                
                if (response.ok) {
                    const serverNote = await response.json();
                    this.notes.set(serverNote.id, serverNote);
                }
            } catch (error) {
                console.error('Failed to create note on server:', error);
                // Note is still saved locally
            }
        }
        
        return note;
    }

    // Edit a note (creates operations)
    editNote(noteId, position, content, isDelete = false) {
        const note = this.notes.get(noteId);
        if (!note) {
            console.error('Note not found:', noteId);
            return;
        }
        
        // Create operation
        const operation = {
            id: this.generateOperationId(),
            client_id: this.clientId,
            clock: this.incrementClock(note.clock),
            type: isDelete ? 'delete' : 'insert',
            position,
            content: isDelete ? '' : content,
            timestamp: new Date().toISOString()
        };
        
        // Apply locally
        this.applyOperation(note, operation);
        
        // Save locally
        this.saveToLocalStorage();
        
        // If online, send to server
        if (this.isOnline && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify({
                type: 'edit',
                payload: {
                    note_id: noteId,
                    client_id: this.clientId,
                    operation
                }
            }));
        } else {
            // Queue for later sync
            this.pendingOperations.push({ noteId, operation });
        }
    }

    // Apply operation to local CRDT
    applyOperation(note, operation) {
        note.crdt.operations.push(operation);
        note.clock = operation.clock;
        note.content = this.rebuildText(note.crdt);
        note.updated_at = new Date().toISOString();
        
        if (this.onNoteUpdate) {
            this.onNoteUpdate(note);
        }
    }

    // Apply remote edit
    applyRemoteEdit(edit) {
        const note = this.notes.get(edit.note_id);
        if (!note) return;
        
        // Don't apply our own operations
        if (edit.client_id === this.clientId) return;
        
        this.applyOperation(note, edit.operation);
        this.saveToLocalStorage();
    }

    // Sync with server
    async sync() {
        if (!this.isOnline) {
            console.log('Cannot sync while offline');
            return;
        }
        
        const syncRequest = {
            client_id: this.clientId,
            notes: Array.from(this.notes.values()),
            last_sync: Object.fromEntries(this.lastSync)
        };
        
        try {
            const response = await fetch(`${this.serverUrl}/api/sync`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(syncRequest)
            });
            
            if (response.ok) {
                const syncResponse = await response.json();
                this.handleSyncResponse(syncResponse);
            }
        } catch (error) {
            console.error('Sync failed:', error);
        }
    }

    // Handle sync response
    handleSyncResponse(response) {
        // Update notes from server
        for (const serverNote of response.notes) {
            const localNote = this.notes.get(serverNote.id);
            
            if (!localNote) {
                // New note from server
                this.notes.set(serverNote.id, serverNote);
            } else {
                // Merge if needed (server already did conflict resolution)
                this.notes.set(serverNote.id, serverNote);
            }
            
            // Update last sync clock
            if (response.clock[serverNote.id]) {
                this.lastSync.set(serverNote.id, response.clock[serverNote.id]);
            }
        }
        
        // Handle conflicts
        if (response.conflicts && response.conflicts.length > 0) {
            console.log('Conflicts detected:', response.conflicts);
            if (this.onConflict) {
                this.onConflict(response.conflicts);
            }
        }
        
        // Clear pending operations
        this.pendingOperations = [];
        
        // Save to local storage
        this.saveToLocalStorage();
        
        // Notify updates
        if (this.onNoteUpdate) {
            this.notes.forEach(note => this.onNoteUpdate(note));
        }
    }

    // Helper: Create CRDT structure
    createCRDT(initialContent) {
        return {
            operations: [],
            text: initialContent,
            clock: { [this.clientId]: 0 }
        };
    }

    // Helper: Increment vector clock
    incrementClock(clock) {
        const newClock = { ...clock };
        newClock[this.clientId] = (newClock[this.clientId] || 0) + 1;
        return newClock;
    }

    // Helper: Rebuild text from operations
    rebuildText(crdt) {
        const ops = [...crdt.operations].sort((a, b) => {
            // Simple timestamp-based ordering for demo
            return new Date(a.timestamp) - new Date(b.timestamp);
        });
        
        let text = '';
        for (const op of ops) {
            if (op.type === 'insert') {
                text = text.slice(0, op.position) + op.content + text.slice(op.position);
            } else if (op.type === 'delete' && op.position < text.length) {
                text = text.slice(0, op.position) + text.slice(op.position + 1);
            }
        }
        
        return text;
    }

    // Helper: Generate IDs
    generateNoteId() {
        return 'note-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9);
    }

    generateOperationId() {
        return 'op-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9);
    }

    // Local storage persistence
    saveToLocalStorage() {
        const data = {
            notes: Array.from(this.notes.entries()),
            lastSync: Array.from(this.lastSync.entries()),
            pendingOperations: this.pendingOperations
        };
        localStorage.setItem(`notes-sync-${this.clientId}`, JSON.stringify(data));
    }

    loadFromLocalStorage() {
        const data = localStorage.getItem(`notes-sync-${this.clientId}`);
        if (data) {
            const parsed = JSON.parse(data);
            this.notes = new Map(parsed.notes);
            this.lastSync = new Map(parsed.lastSync);
            this.pendingOperations = parsed.pendingOperations || [];
        }
    }

    // Get all notes
    getAllNotes() {
        return Array.from(this.notes.values());
    }

    // Get a single note
    getNote(noteId) {
        return this.notes.get(noteId);
    }
}

// Example usage:
/*
const client = new NotesSyncClient('http://localhost:8080', 'my-client-id');

// Set up event handlers
client.onNoteUpdate = (note) => {
    console.log('Note updated:', note);
    // Update UI
};

client.onConflict = (conflicts) => {
    console.log('Conflicts resolved:', conflicts);
    // Show notification
};

client.onConnectionChange = (isOnline) => {
    console.log('Connection status:', isOnline ? 'Online' : 'Offline');
    // Update UI indicator
};

// Load cached data
client.loadFromLocalStorage();

// Connect to server
await client.connect();

// Create a note
const note = await client.createNote('My Note', 'Initial content');

// Edit the note
client.editNote(note.id, 0, 'Hello '); // Insert at position 0

// The client automatically syncs when online
// and queues operations when offline
*/

// Export for use in modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = NotesSyncClient;
}
