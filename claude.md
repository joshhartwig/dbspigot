# Claude Instructions — DBSpigot

## Project Overview

DBSpigot is a small self-hosted web app that acts like a **database faucet / spigot**.

Users click a button and instantly get a fresh PostgreSQL database. With a connection string that they can copy and paste right from the main screen.

Think:

turn handle → database flows out

The app:
- creates Postgres containers on demand
- returns a connection string
- lists running instances
- allows deleting instances
- runs fully locally in a homelab
- has ZERO cloud dependencies

This is intentionally simple and lightweight.

This is a tool, not a platform.


---

## Tech Stack (DO NOT change unless explicitly asked)

Backend:
- Go (latest stable)
- standard library http
- templ for HTML rendering

Container control:
- Docker Engine
- Docker Go SDK (github.com/moby/moby/client)

Deployment:
- Docker Compose
- single container
- mounts /var/run/docker.sock

No:
- React
- SPA frameworks
- ORMs
- external databases
- Kubernetes
- heavy dependencies


---

## Core Features

DBSpigot MUST support:

### 1. Create database
- start new postgres container
- random user
- random password
- random host port mapping
- return connection string:

postgresql://user:pass@host:port/db

### 2. List databases
- list containers using label:
  homelab.dbspigot=true
- display:
  id
  container name
  port
  created time

### 3. Delete database
- stop container
- remove container
- remove volumes

### 4. Basic auth
- protect entire app with HTTP basic auth
- credentials from environment variables


---

## Architecture Rules

### Philosophy

Keep it:
- tiny
- readable
- hackable
- homelab friendly

Prefer:
simple > clever
explicit > abstract
small > generic

Avoid:
- heavy patterns
- unnecessary layers
- deep abstractions
- "enterprise" solutions

This should feel like a weekend project you can understand in 10 minutes.


---

## Code Structure

Follow this layout:

```
cmd/server/main.go
internal/docker/docker.go
internal/web/handlers.go
internal/web/views/*.templ
Dockerfile
docker-compose.yml
claude.md
```

Do not add extra folders or complexity unless necessary.


---

## Docker Rules

### Labels (REQUIRED)

Every created container must include:

- homelab.dbspigot=true
- homelab.id=<short-id>
- homelab.db=<dbname>

Labels are the ONLY source of truth.

Do NOT store state in:
- files
- sqlite
- redis
- memory caches

Docker itself is the database of truth.


### Port mapping
- publish 5432 → RANDOM host port
- never hardcode ports
- inspect container to retrieve assigned port


### Resource limits
Containers should have:
- memory ~512MB
- cpu ~1 core

Prevents runaway resource usage.


---

## Go Docker SDK Rules

Use:

github.com/moby/moby/client

NOT:
github.com/docker/docker/client

Use modern options-based APIs:
- ContainerCreateOptions
- ContainerStartOptions
- ContainerInspect
- ContainerRemoveOptions
- NetworkCreateOptions

Always:
- use context with timeouts
- read ImagePull stream to completion


---

## templ Rules

templ is for simple server-rendered HTML only.

Allowed:
- forms
- tables
- minimal CSS
- progressive enhancement

Avoid:
- SPA behavior
- heavy JavaScript
- frontend frameworks

UI should be extremely basic:

button → connection string → table


---

## Security Assumptions

Runs:
- on LAN
- behind VPN or private network
- not public internet

Therefore:
- simple basic auth is sufficient
- no OAuth/JWT required
- no complex auth systems


---

## Naming & Style

Theme: mechanical / faucet / spigot / industrial

Tone:
- practical
- minimal
- dev-tool vibe

Avoid:
- buzzwords
- corporate naming
- marketing fluff


---

## How Claude Should Modify This Project

When making changes:
- touch the smallest amount of code possible
- keep functions short
- avoid adding dependencies
- prefer Docker-native solutions
- keep everything obvious

When unsure:
choose the simplest solution.


---

## What Claude MUST NOT Do

Do NOT:
- add React or SPA frameworks
- add Tailwind or heavy UI libs
- add a database for metadata
- add Redis
- add background workers
- introduce Kubernetes
- create microservices
- add complex patterns

This must remain:

single Go binary + Docker


---

## Desired UX

Target experience:

1. open browser
2. click "Create DB"
3. connection string appears
4. copy
5. use immediately
6. delete when done

Everything should feel instant and frictionless.


---

## Optional Future Features (only if requested)

Possible:
- TTL auto-delete
- pause/resume containers
- named instances
- template DB seed
- multiple Docker hosts
- usage limits

Do NOT implement unless explicitly asked.


---

## Final Principle

If it feels overengineered,
it's wrong.

DBSpigot should feel like:

curl → get database

Small. Fast. Fun. Hackable.
