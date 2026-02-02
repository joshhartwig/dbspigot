package docker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const (
	postgresImage = "postgres:16-alpine"
	labelSpigot   = "homelab.dbspigot"
	labelID       = "homelab.id"
	labelDB       = "homelab.db"
)

type Client struct {
	docker *client.Client
}

type Database struct {
	ID               string
	ContainerID      string
	ContainerName    string
	DBName           string
	User             string
	Password         string
	Host             string
	Port             string
	Created          time.Time
	ConnectionString string
}

func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &Client{docker: cli}, nil
}

func (c *Client) Close() error {
	return c.docker.Close()
}

func randomString(n int) string {
	bytes := make([]byte, n)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:n]
}

func (c *Client) CreateDatabase(ctx context.Context, host string) (*Database, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Pull image if not present
	reader, err := c.docker.ImagePull(ctx, postgresImage, image.PullOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to pull image: %w", err)
	}
	// Must read to completion
	io.Copy(io.Discard, reader)
	reader.Close()

	// Generate random credentials
	shortID := randomString(8)
	dbName := "db_" + randomString(6)
	user := "user_" + randomString(6)
	password := randomString(16)

	containerName := "dbspigot-" + shortID

	// Container configuration
	containerConfig := &container.Config{
		Image: postgresImage,
		Env: []string{
			"POSTGRES_DB=" + dbName,
			"POSTGRES_USER=" + user,
			"POSTGRES_PASSWORD=" + password,
		},
		Labels: map[string]string{
			labelSpigot: "true",
			labelID:     shortID,
			labelDB:     dbName,
		},
		ExposedPorts: nat.PortSet{
			"5432/tcp": struct{}{},
		},
	}

	// Host configuration with resource limits
	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			"5432/tcp": []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: ""}, // Random port
			},
		},
		Resources: container.Resources{
			Memory:   512 * 1024 * 1024, // 512MB
			NanoCPUs: 1000000000,        // 1 CPU
		},
	}

	// Create container
	resp, err := c.docker.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := c.docker.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		// Clean up on failure
		c.docker.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Inspect to get assigned port
	inspect, err := c.docker.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	portBindings := inspect.NetworkSettings.Ports["5432/tcp"]
	if len(portBindings) == 0 {
		return nil, fmt.Errorf("no port binding found")
	}
	hostPort := portBindings[0].HostPort

	db := &Database{
		ID:            shortID,
		ContainerID:   resp.ID,
		ContainerName: containerName,
		DBName:        dbName,
		User:          user,
		Password:      password,
		Host:          host,
		Port:          hostPort,
		Created:       time.Now(),
	}
	db.ConnectionString = fmt.Sprintf("postgresql://%s:%s@%s:%s/%s", user, password, host, hostPort, dbName)

	return db, nil
}

func (c *Client) ListDatabases(ctx context.Context, host string) ([]Database, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	filter := filters.NewArgs()
	filter.Add("label", labelSpigot+"=true")

	containers, err := c.docker.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filter,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var databases []Database
	for _, cont := range containers {
		// Inspect for full details
		inspect, err := c.docker.ContainerInspect(ctx, cont.ID)
		if err != nil {
			continue
		}

		shortID := cont.Labels[labelID]
		dbName := cont.Labels[labelDB]

		// Get port
		var hostPort string
		if portBindings, ok := inspect.NetworkSettings.Ports["5432/tcp"]; ok && len(portBindings) > 0 {
			hostPort = portBindings[0].HostPort
		}

		// Get env vars for credentials
		var user, password string
		for _, env := range inspect.Config.Env {
			if len(env) > 14 && env[:14] == "POSTGRES_USER=" {
				user = env[14:]
			}
			if len(env) > 18 && env[:18] == "POSTGRES_PASSWORD=" {
				password = env[18:]
			}
		}

		db := Database{
			ID:            shortID,
			ContainerID:   cont.ID,
			ContainerName: cont.Names[0][1:], // Remove leading /
			DBName:        dbName,
			User:          user,
			Password:      password,
			Host:          host,
			Port:          hostPort,
			Created:       time.Unix(cont.Created, 0),
		}
		if hostPort != "" && user != "" && password != "" {
			db.ConnectionString = fmt.Sprintf("postgresql://%s:%s@%s:%s/%s", user, password, host, hostPort, dbName)
		}

		databases = append(databases, db)
	}

	return databases, nil
}

func (c *Client) DeleteDatabase(ctx context.Context, shortID string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Find container by label
	filter := filters.NewArgs()
	filter.Add("label", labelID+"="+shortID)

	containers, err := c.docker.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filter,
	})
	if err != nil {
		return fmt.Errorf("failed to find container: %w", err)
	}

	if len(containers) == 0 {
		return fmt.Errorf("container not found: %s", shortID)
	}

	containerID := containers[0].ID

	// Stop container (with timeout)
	stopTimeout := 10
	if err := c.docker.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &stopTimeout}); err != nil {
		// Continue even if stop fails (might already be stopped)
	}

	// Remove container with volumes
	if err := c.docker.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	return nil
}
