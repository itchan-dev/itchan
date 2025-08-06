
# Itchan Imageboard

A minimal, fast imageboard implemented in Go with a clear three-layer backend (storage → service → handler) and a Go-template frontend. Deployed using Docker Compose.

## Table of Contents

- Features
- Architecture
- Getting Started
- Project Structure
- Backend Layers
- Frontend
- Testing
- Contributing
- License

## Features

- Plain HTML + Go templates, no heavy JS  
- Fast, concurrent Go server  
- Simple routing with Gorilla Mux  
- Pagination, bump limits, replies  
- JWT-based authentication  
- Dockerized frontend & backend  

## Architecture

```
docker-compose.yml
├── backend/           # API server
│   ├── internal/
│   │   ├── storage/pg
│   │   │   └── board.go           (#Storage.CreateBoard, #Storage.GetBoard, #Storage.DeleteBoard)
│   │   │   └── message.go         (#Storage.CreateMessage, #Storage.GetMessage, #Storage.DeleteMessage)
│   │   ├── service
│   │   │   └── board.go           (service.Board)
│   │   │   └── thread.go          (service.Thread)
│   │   └── handler
│   │       └── board.go           (Handler.CreateBoard, Handler.GetBoard…)
│   │       └── message.go         (Handler.CreateMessage etc.)
│   └── cmd/itchan-api/main.go     (entrypoint)
└── frontend/          # UI server
    ├── internal/
    │   ├── handler
    │   │   └── board.go            (Handler.BoardGetHandler, Handler.BoardPostHandler)
    │   │   └── thread.go           (Handler.ThreadGetHandler…)
    │   │   └── helpers.go          (RenderThreadWithReplies, redirectWithError)
    │   └── setup/setup.go          (template loader, JWT deps)
    ├── templates/                   (HTML templates: base.html, index.html, board.html…)
    └── cmd/frontend/main.go        (entrypoint)
```

Refer to docker-compose.yml to wire services.

## Getting Started

### Prerequisites

- Docker & Docker Compose  
- Go 1.24+ (for local development)

### Running Locally

```sh
git clone https://github.com/itchan-dev/itchan.git
cd itchan
docker-compose up --build
```

- Backend API: http://localhost:8080/v1  
- Frontend UI: http://localhost:8081  

### Manual Go Run (no Docker)

```sh
# Backend
cd backend
go run cmd/itchan-api/main.go -config_folder ./config

# Frontend
cd frontend
go run cmd/frontend/main.go
```

## Backend Layers

1. **Storage** (pg)  
   - Implements raw SQL operations and partition logic.  
   - Key files:  
     - board.go (`Storage.CreateBoard`, `Storage.GetBoard`, `Storage.DeleteBoard`)  
     - message.go (`Storage.CreateMessage`, `Storage.GetMessage`, `Storage.DeleteMessage`)  

2. **Service** (service)  
   - Business rules, validation, bump limits.  
   - Key files:  
     - board.go (`service.NewBoard`, `Board.Create`, `Board.Get`)  
     - thread.go (`service.NewThread`)  

3. **Handler** (handler)  
   - HTTP endpoints, error mapping, JSON I/O.  
   - Key files:  
     - board.go (`Handler.CreateBoard`, `Handler.GetBoard`)  
     - message.go  

## Frontend

- Uses Go’s `html/template` in handler to render pages.  
- Template files under templates:  
  - base.html: layout & navigation  
  - index.html: list & create boards (with `Handler.IndexGetHandler`)  
  - board.html, `thread.html`: thread and message UI  

- Entrypoint: cmd/frontend/main.go  

## Testing

```sh
# Backend unit & integration
cd backend
go test ./internal/storage/... ./internal/service/... ./internal/handler/...

# Frontend handlers
cd frontend
go test ./internal/handler/...
```

Integration tests for storage are in integration_board_test.go.

## Contributing

1. Fork & clone  
2. Create feature branch  
3. Ensure tests pass  
4. Submit PR  

## License

MIT &copy; 2023 Itchan Imageboard  
See LICENSE for details.