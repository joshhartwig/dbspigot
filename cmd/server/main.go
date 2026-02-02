package main

import (
	"log"
	"net/http"
	"os"

	"dbspigot/internal/docker"
	"dbspigot/internal/web"
)

func main() {
	// Load configuration from environment
	username := os.Getenv("DBSPIGOT_USER")
	password := os.Getenv("DBSPIGOT_PASS")
	host := os.Getenv("DBSPIGOT_HOST")
	port := os.Getenv("DBSPIGOT_PORT")

	if username == "" || password == "" {
		log.Fatal("DBSPIGOT_USER and DBSPIGOT_PASS environment variables are required")
	}

	if host == "" {
		host = "localhost"
	}

	if port == "" {
		port = "8181"
	}

	// Initialize Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}
	defer dockerClient.Close()

	// Set up HTTP server
	handler := web.NewHandler(dockerClient, username, password, host)
	mux := http.NewServeMux()
	handler.SetupRoutes(mux)

	addr := ":" + port
	log.Printf("Starting DBSpigot on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
