# Mimir - Your Personal Knowledge Base

[![Go Report Card](https://goreportcard.com/badge/github.com/dotcommander/mimir)](https://goreportcard.com/report/github.com/dotcommander/mimir)
[![Go Version](https://img.shields.io/github/go-mod/go-version/dotcommander/mimir)](go.mod)
<!-- Add other badges as needed: build status, license, etc. -->

Mimir is a Go-powered personal knowledge base designed for efficient information capture, organization, and retrieval. It features a robust CLI, a planned REST API, hybrid search capabilities, asynchronous processing for AI tasks, and support for multiple AI providers.

**Status:** ⚠️ **Experimental** ⚠️ - This application is under active development. While core features are functional, expect potential bugs, breaking changes, and incomplete features. See the [Status & Roadmap](#status--roadmap-current) section below for current progress and next steps.


## Features

- **Ingest Anything:** Add text, PDFs, URLs, local files, or entire directories.
- **Smart Categorization:** Automatically suggests and applies tags using AI (configurable LLM providers).
- **Hybrid Search:** Combines traditional keyword search with semantic vector search for highly relevant results.
- **Retrieval-Augmented Generation (RAG):** Generate answers to questions based on the content within your knowledge base using LLMs (powered by Gemini).
- **Multiple AI Providers:** Supports OpenAI, Gemini, Anthropic (and potentially local models) with configurable fallback and retry strategies for embeddings and categorization.
- **Asynchronous Pipeline:** Uses Redis and background workers for efficient processing of embeddings and other AI tasks without blocking ingestion.
- **Pure Go:** Built entirely in Go, ensuring portability and minimal external dependencies (no Python/Java required).
- **Organize Your Way:** Use tags and collections to structure your knowledge base.
- **CLI & API Access:** Interact via a comprehensive command-line interface or integrate with other applications using the REST API.

## Architecture

Mimir follows a layered architecture:

1.  **Configuration (`config.yaml`, `internal/config`):** Manages deployment settings.
2.  **Application (`internal/app`):** Central hub holding initialized components (`App` struct).
3.  **Stores (`internal/store/*`):** Data access layer (PostgreSQL for primary data, PostgreSQL+pgvector for vectors, Redis for jobs).
4.  **Services (`internal/services`):** Core business logic (Content, Search, Embedding, Categorization, RAG, etc.).
5.  **Commands (`cmd`):** CLI interface (Cobra), directly calling services.
6.  **Worker (`internal/worker`, `cmd/worker`):** Background job processor (Asynq).
7.  **API Handlers (`internal/apihandlers`, `cmd/serve`):** REST API layer (Gin).

*(See `SPECS.md` for more architectural details and coding guidelines)*

## Prerequisites

- Go 1.20+
- PostgreSQL (with the `pgvector` extension enabled)
- Redis

## Getting Started

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/dotcommander/mimir.git
    cd mimir
    ```
2.  **Set up Databases:**
    - Ensure PostgreSQL is running and accessible.
    - Create a database (e.g., `mimir_db`).
    - Enable the `pgvector` extension: `CREATE EXTENSION IF NOT EXISTS vector;`
    - Create a user (e.g., `mimir_user`) with a password and grant privileges on the database.
    - Ensure Redis is running and accessible.
3.  **Configure:**
    - Copy the example configuration: `cp config.example.yaml config.yaml`
    - Edit `config.yaml` to set your database DSNs (for primary and vector stores), Redis connection details, and API keys for desired AI providers (OpenAI, Gemini, etc.).
    - **Important:** It is **strongly recommended** to set sensitive values (DSNs, API keys) via **environment variables** (e.g., `PRIMARY_DB_DSN`, `VECTOR_DB_DSN`, `OPENAI_API_KEY`, `GEMINI_API_KEY`). The configuration file supports environment variable substitution using the `${VAR_NAME:-default}` syntax. See `config.example.yaml` for details.
    - Ensure `config.yaml` is added to your `.gitignore` file to avoid committing secrets.
4.  **Build the application:**
    ```bash
    go build -o mimir ./cmd # Build the main CLI application
    ```
5.  **Build the worker:** The worker is essential for background processing (embeddings, etc.).
    ```bash
    go build -o worker ./cmd/worker
    ```
6.  **(Optional) Build the API server:**
    ```bash
    go build -o serve ./cmd/serve
    ```
7.  **Run Database Migrations:** (Assuming migrations are managed, e.g., with `migrate` or similar - *TODO: Add migration details if applicable*)
    ```bash
    # Example using golang-migrate (adjust command as needed)
    # migrate -database "postgres://user:pass@host:port/db?sslmode=disable" -path ./migrations up
    ```

## Usage

### Command Line Interface (CLI)

The `mimir` executable is the main entry point for interacting with your knowledge base.

```bash
# Get help on any command
./mimir --help
./mimir add --help

# Add content (URL, file, or directory)
./mimir add "https://example.com/article" --title "Example Article" --tags "web,example"
./mimir add ./my_document.pdf --collection "research-papers"
./mimir add ./notes/ --recursive # Add all files in the notes directory

# Search content (combines keyword and semantic search)
./mimir search "machine learning techniques" --limit 5

# Ask a question using RAG
./mimir answer "What are the main differences between supervised and unsupervised learning based on my documents?"

# List content with filters
./mimir list --limit 20 --tags "web,example" --sort-by created_at --sort-order desc

# Manage collections
./mimir collection create --name "Project X" --description "Documents related to Project X"
./mimir collection add --collection-id 1 --content-id 5
./mimir collection list

# Manage tags
./mimir tag 5 "important" "urgent" # Add tags 'important' and 'urgent' to content ID 5
./mimir tag list

# Manage background jobs (view status)
./mimir batch list
```

### Background Worker

The worker process handles asynchronous tasks like generating embeddings and performing AI categorization. It needs to be running for these features to work.

```bash
# Run the background worker in the foreground
./worker

# Or run it as a background service using systemd, supervisor, etc.
```

### REST API

Mimir also provides a REST API for programmatic access.

```bash
# Run the API server (if built/enabled)
./serve

# Example API call (details may vary - TODO: Link to API docs)
curl -X POST http://localhost:8080/api/v1/search -H "Content-Type: application/json" -d '{"query": "data privacy laws", "limit": 5}'
```

## Development

- **Build:** `go build -o mimir ./cmd`
- **Build Worker:** `go build -o worker ./cmd/worker`
- **Build API Server:** `go build -o serve ./cmd/serve`
- **Run CLI:** `./mimir <command>`
- **Run Worker:** `go run ./cmd/worker` or `./worker` (after building)
- **Run API Server:** `go run ./cmd/serve` or `./serve` (after building)
- **Lint:** `go fmt ./...` and `go vet ./...`
- **Test All:** `go test ./...`
- **Test Single Package:** `go test ./internal/services`

Refer to `SPECS.md` for detailed style guidelines and architectural overview.

## Configuration

Mimir is configured via `config.yaml`. Key sections include:
- `database`: Connection details for primary (PostgreSQL) and vector (PostgreSQL+pgvector) databases.
- `embedding`: Configuration for AI providers (OpenAI, Gemini, etc.), including API keys, models, and fallback strategy.
- `redis`: Connection details for the Redis instance used by the background job queue.
- `worker`: Settings for the background worker, including concurrency and queue priorities.
- `categorization`: Configuration for the LLM-based categorization service.
- `pricing`: Optional cost definitions for different AI models used for tracking.
- `rag`: Settings for the Retrieval-Augmented Generation feature, including the completion provider and prompt template.

**Environment Variables:** As mentioned in setup, using environment variables (e.g., `${PRIMARY_DB_DSN}`) is the recommended way to handle sensitive data like DSNs and API keys. Refer to `config.example.yaml` for examples.

## Status & Roadmap (Current)

**Core Features Implemented:**
- File/URL/Directory ingestion & processing
- Hybrid search (keyword + vector)
- AI categorization (via LLMs)
- Multi-provider embeddings (OpenAI, Gemini, Anthropic)
- Background job processing (Redis/Asynq) for embeddings and categorization
- Retrieval-Augmented Generation (RAG) via Gemini

**Next Steps:**
- **Complete Test Suite:** Implement comprehensive tests, especially for services (`RAGService`, `GeminiProvider` generation), CLI commands (`answer`), and API handlers (`answer`). (Target: 85% coverage)
- Refine RAG prompt engineering and context handling.
- Optimize vector indexing performance.
- Finalize API design and documentation.
- Performance benchmarking.
- Add database migration tooling/process.
- Prepare v1.0 release artifacts.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request. Follow the guidelines in `SPECS.md`.

## License

This project is licensed under the [MIT License](LICENSE). <!-- TODO: Add LICENSE file -->
# mimir
