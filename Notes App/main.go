package main

import (
	"log"
	"net/http"
	"os"

	"github.com/yourusername/notes-sync-backend/internal/api"
	"github.com/yourusername/notes-sync-backend/internal/store"
	"github.com/yourusername/notes-sync-backend/internal/sync"
)

func main() {
	// Get configuration from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./notes.db"
	}

	// Initialize store
	log.Printf("Initializing database at %s", dbPath)
	dataStore, err := store.NewStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}
	defer dataStore.Close()

	// Initialize sync engine
	syncEngine := sync.NewEngine(dataStore)

	// Initialize API server
	apiServer := api.NewServer(syncEngine)
	apiServer.Start()

	// Set up routes
	http.HandleFunc("/api/sync", api.EnableCORS(apiServer.HandleSync))
	http.HandleFunc("/api/notes", api.EnableCORS(apiServer.HandleGetNotes))
	http.HandleFunc("/api/notes/create", api.EnableCORS(apiServer.HandleCreateNote))
	http.HandleFunc("/ws", apiServer.HandleWebSocket)

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Serve static files for a simple demo UI (optional)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(demoHTML))
		} else {
			http.NotFound(w, r)
		}
	})

	log.Printf("Server starting on port %s", port)
	log.Printf("WebSocket endpoint: ws://localhost:%s/ws", port)
	log.Printf("HTTP API endpoints:")
	log.Printf("  POST /api/sync - Sync notes")
	log.Printf("  GET  /api/notes - Get all notes")
	log.Printf("  POST /api/notes/create - Create new note")
	log.Printf("  GET  /health - Health check")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

const demoHTML = `<!DOCTYPE html>
<html>
<head>
    <title>Notes Sync Backend - Demo</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            background: #f5f5f5;
        }
        h1 { color: #333; }
        .container {
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 20px;
        }
        .status {
            padding: 10px;
            border-radius: 4px;
            margin-bottom: 10px;
        }
        .status.connected { background: #d4edda; color: #155724; }
        .status.disconnected { background: #f8d7da; color: #721c24; }
        button {
            background: #007bff;
            color: white;
            border: none;
            padding: 10px 20px;
            border-radius: 4px;
            cursor: pointer;
            margin: 5px;
        }
        button:hover { background: #0056b3; }
        textarea {
            width: 100%;
            min-height: 100px;
            padding: 10px;
            border: 1px solid #ddd;
            border-radius: 4px;
            font-family: monospace;
            box-sizing: border-box;
        }
        input {
            width: 100%;
            padding: 10px;
            border: 1px solid #ddd;
            border-radius: 4px;
            margin-bottom: 10px;
            box-sizing: border-box;
        }
        .note {
            border: 1px solid #ddd;
            padding: 15px;
            margin: 10px 0;
            border-radius: 4px;
            background: #fafafa;
        }
        .note h3 { margin-top: 0; }
        pre {
            background: #f8f9fa;
            padding: 10px;
            border-radius: 4px;
            overflow-x: auto;
        }
        .info { color: #666; font-size: 0.9em; }
    </style>
</head>
<body>
    <h1>📝 Notes Sync Backend - Demo</h1>
    
    <div class="container">
        <h2>Connection Status</h2>
        <div id="status" class="status disconnected">Disconnected</div>
        <button onclick="connect()">Connect WebSocket</button>
        <button onclick="disconnect()">Disconnect</button>
    </div>

    <div class="container">
        <h2>Create Note</h2>
        <input type="text" id="noteTitle" placeholder="Note title">
        <textarea id="noteContent" placeholder="Note content"></textarea>
        <button onclick="createNote()">Create Note</button>
    </div>

    <div class="container">
        <h2>Notes</h2>
        <button onclick="loadNotes()">Refresh Notes</button>
        <div id="notes"></div>
    </div>

    <div class="container">
        <h2>API Information</h2>
        <pre>
<strong>REST Endpoints:</strong>
POST /api/sync          - Sync notes with conflict resolution
GET  /api/notes         - Get all notes
POST /api/notes/create  - Create a new note
GET  /health            - Health check

<strong>WebSocket:</strong>
ws://localhost:8080/ws?client_id=YOUR_CLIENT_ID

<strong>Message Types:</strong>
- edit: Real-time edit operations
- sync: Sync request/response
        </pre>
    </div>

    <script>
        let ws = null;
        const clientId = 'client-' + Math.random().toString(36).substr(2, 9);

        function connect() {
            ws = new WebSocket('ws://localhost:8080/ws?client_id=' + clientId);
            
            ws.onopen = () => {
                document.getElementById('status').textContent = 'Connected (Client: ' + clientId + ')';
                document.getElementById('status').className = 'status connected';
                console.log('WebSocket connected');
            };
            
            ws.onclose = () => {
                document.getElementById('status').textContent = 'Disconnected';
                document.getElementById('status').className = 'status disconnected';
                console.log('WebSocket disconnected');
            };
            
            ws.onmessage = (event) => {
                const msg = JSON.parse(event.data);
                console.log('Received:', msg);
                if (msg.type === 'edit') {
                    console.log('Real-time edit received:', msg.payload);
                    loadNotes();
                }
            };
            
            ws.onerror = (error) => {
                console.error('WebSocket error:', error);
            };
        }

        function disconnect() {
            if (ws) {
                ws.close();
                ws = null;
            }
        }

        async function createNote() {
            const title = document.getElementById('noteTitle').value;
            const content = document.getElementById('noteContent').value;
            
            const response = await fetch('/api/notes/create', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ title, content, client_id: clientId })
            });
            
            if (response.ok) {
                const note = await response.json();
                console.log('Created note:', note);
                document.getElementById('noteTitle').value = '';
                document.getElementById('noteContent').value = '';
                loadNotes();
            }
        }

        async function loadNotes() {
            const response = await fetch('/api/notes');
            const notes = await response.json();
            
            const notesDiv = document.getElementById('notes');
            notesDiv.innerHTML = notes.map(note => `
                <div class="note">
                    <h3>${note.title}</h3>
                    <p>${note.content}</p>
                    <div class="info">
                        ID: ${note.id}<br>
                        Created: ${new Date(note.created_at).toLocaleString()}<br>
                        Updated: ${new Date(note.updated_at).toLocaleString()}
                    </div>
                </div>
            `).join('') || '<p>No notes yet. Create one above!</p>';
        }

        // Load notes on page load
        loadNotes();
    </script>
</body>
</html>`
