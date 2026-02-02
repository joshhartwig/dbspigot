package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"dbspigot/internal/docker"
	"dbspigot/internal/web/views"
)

type Handler struct {
	docker    *docker.Client
	username  string
	password  string
	host      string
	localAuth bool
}

func NewHandler(dockerClient *docker.Client, username, password, host string, localAuth bool) *Handler {
	return &Handler{
		docker:    dockerClient,
		username:  username,
		password:  password,
		host:      host,
		localAuth: localAuth,
	}
}

func (h *Handler) basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.localAuth {
			next(w, r)
			return
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != h.username || pass != h.password {
			w.Header().Set("WWW-Authenticate", `Basic realm="DBSpigot"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (h *Handler) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /", h.basicAuth(h.handleIndex))
	mux.HandleFunc("POST /create", h.basicAuth(h.handleCreate))
	mux.HandleFunc("POST /delete/{id}", h.basicAuth(h.handleDelete))
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	databases, err := h.docker.ListDatabases(r.Context(), h.host)
	if err != nil {
		log.Printf("Error listing databases: %v", err)
		http.Error(w, "Failed to list databases", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.Index(databases).Render(r.Context(), w); err != nil {
		log.Printf("Error rendering template: %v", err)
	}
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	db, err := h.docker.CreateDatabase(context.Background(), h.host)
	if err != nil {
		log.Printf("Error creating database: %v", err)
		http.Error(w, "Failed to create database", http.StatusInternalServerError)
		return
	}

	if wantsJSON(r) {
		writeDatabaseJSON(w, db)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		// Fallback for older Go versions
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) >= 3 {
			id = parts[2]
		}
	}

	if id == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	if err := h.docker.DeleteDatabase(r.Context(), id); err != nil {
		log.Printf("Error deleting database %s: %v", id, err)
		http.Error(w, "Failed to delete database", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func wantsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return true
	}
	if r.Header.Get("X-Requested-With") == "fetch" {
		return true
	}
	return false
}

func writeDatabaseJSON(w http.ResponseWriter, db *docker.Database) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"id":         db.ID,
		"name":       db.ContainerName,
		"port":       db.Port,
		"connection": db.ConnectionString,
		"created":    db.Created.Format(time.RFC3339),
		"username":   db.User,
		"password":   db.Password,
	})
}
