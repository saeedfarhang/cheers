package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"time"
)

// AppState holds the application's global state.
var appState struct {
	mu           sync.Mutex
	cheersActive bool // True if the main page background should be green
}

// clients map stores a channel for each connected SSE client.
var clients = make(map[chan struct{}]bool)
var clientsMutex sync.Mutex // Mutex to protect the clients map

// tmpl holds our parsed HTML template.
var tmpl *template.Template

func main() {
	// Initialize the state
	appState.cheersActive = false

	// Parse the HTML template once at startup
	var err error
	templatePath := filepath.Join("templates", "index.html")
	tmpl, err = template.ParseFiles(templatePath)
	if err != nil {
		log.Fatalf("Error parsing template %s: %v", templatePath, err)
	}

	// Define handlers
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/cheers", cheersHandler)
	http.HandleFunc("/events", eventsHandler)

	// NEW: Serve static files from the /static/ URL path
	// The http.Dir() creates a filesystem root relative to where the Go app is run.
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))


	// Start the web server
	port := ":8080"
	fmt.Printf("Server starting on http://localhost%s\n", port)
	fmt.Println("Visit http://localhost:8080/ on your monitor.")
	fmt.Println("Trigger animation & sound by visiting http://localhost:8080/cheers from another service/tab.")
	log.Fatal(http.ListenAndServe(port, nil))
}

// indexHandler serves the main page.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	appState.mu.Lock()
	cheersStatus := appState.cheersActive
	appState.mu.Unlock()

	data := struct {
		CheersActive bool
	}{
		CheersActive: cheersStatus,
	}

	err := tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("Error executing template: %v", err)
		return
	}
}

// cheersHandler activates the 'cheers' state and broadcasts the animation event.
func cheersHandler(w http.ResponseWriter, r *http.Request) {
	appState.mu.Lock()
	appState.cheersActive = true // Still sets the background to green
	appState.mu.Unlock()

	// Broadcast the "cheers" event to all connected SSE clients
	clientsMutex.Lock()
	for clientChan := range clients {
		select {
		case clientChan <- struct{}{}: // Send a signal
			log.Println("Sent cheers event to a client.")
		case <-time.After(100 * time.Millisecond): // Avoid blocking indefinitely
			log.Println("Client channel blocked, skipping send.")
		}
	}
	clientsMutex.Unlock()

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Cheers activated and animation/sound event sent to monitor(s)!\n")
}

// eventsHandler manages the Server-Sent Events connection.
func eventsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	clientChan := make(chan struct{})

	clientsMutex.Lock()
	clients[clientChan] = true
	clientsMutex.Unlock()

	log.Println("New SSE client connected.")

	notify := r.Context().Done()
	go func() {
		<-notify
		clientsMutex.Lock()
		delete(clients, clientChan)
		close(clientChan)
		clientsMutex.Unlock()
		log.Println("SSE client disconnected.")
	}()

	for range clientChan {
		fmt.Fprintf(w, "event: cheers\ndata: true\n\n")
		w.(http.Flusher).Flush()
	}
}