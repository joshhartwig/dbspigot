package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

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
	mux.HandleFunc("POST /api/v1/database", h.handlerCreateAPI)
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
	_, err := h.docker.CreateDatabase(context.Background(), h.host)
	if err != nil {
		log.Printf("Error creating database: %v", err)
		http.Error(w, "Failed to create database", http.StatusInternalServerError)
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

func (h *Handler) handlerCreateAPI(w http.ResponseWriter, r *http.Request) {
	db, err := h.docker.CreateDatabase(r.Context(), h.host)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"Failed to create database"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(db)
	if err != nil {
		http.Error(w, "Failed to delete database", http.StatusInternalServerError)
	}
}
