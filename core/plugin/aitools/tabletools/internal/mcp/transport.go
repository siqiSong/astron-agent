package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sync"
)

type sessionStore struct {
	mu     sync.RWMutex
	values map[string]chan []byte
}

func newSessionStore() *sessionStore { return &sessionStore{values: map[string]chan []byte{}} }
func (store *sessionStore) create() (string, chan []byte) {
	raw := make([]byte, 16)
	rand.Read(raw)
	id := hex.EncodeToString(raw)
	channel := make(chan []byte, 8)
	store.mu.Lock()
	store.values[id] = channel
	store.mu.Unlock()
	return id, channel
}
func (store *sessionStore) get(id string) (chan []byte, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	value, ok := store.values[id]
	return value, ok
}
func (store *sessionStore) remove(id string) {
	store.mu.Lock()
	delete(store.values, id)
	store.mu.Unlock()
}

func (server *Server) HandleStreamable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	response, notification := server.Dispatch(body)
	if notification {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}
func (server *Server) HandleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	id, channel := server.sessions.create()
	defer server.sessions.remove(id)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	fmt.Fprintf(w, "event: endpoint\ndata: /mcp/messages?session_id=%s\n\n", id)
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case response := <-channel:
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", response)
			flusher.Flush()
		}
	}
}
func (server *Server) HandleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	channel, ok := server.sessions.get(r.URL.Query().Get("session_id"))
	if !ok {
		http.Error(w, "unknown session", 404)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	response, notification := server.Dispatch(body)
	if !notification {
		select {
		case channel <- response:
		default:
			http.Error(w, "session busy", 429)
			return
		}
	}
	w.WriteHeader(http.StatusAccepted)
}
