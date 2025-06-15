package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for now, but you should restrict this in production
		return true
	},
}

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan Message)

type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

func main() {
	// Set up logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting server...")

	// Serve static files
	fs := http.FileServer(http.Dir("."))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Serve index.html at root
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request for path: %s", r.URL.Path)
		if r.URL.Path == "/" {
			http.ServeFile(w, r, "index.html")
		} else {
			fs.ServeHTTP(w, r)
		}
	})

	// WebSocket endpoint
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received WebSocket request from: %s", r.RemoteAddr)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		handleConnections(w, r)
	})

	go handleMessages()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading connection: %v", err)
		return
	}
	defer conn.Close()

	clients[conn] = true
	log.Printf("New client connected. Total clients: %d", len(clients))

	for {
		var msg Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("Error reading message: %v", err)
			delete(clients, conn)
			return
		}

		broadcast <- msg
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
		log.Printf("Broadcasting message from %s", msg.Username)

		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("Error writing message: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}
