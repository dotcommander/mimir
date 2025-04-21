# Mimir Documentation

## Overview
Mimir is a Go-powered personal knowlege base with CLI + REST interfaces. Features hybrid search (semantic/keyword), async embeddings, and pure Go parsers.

## Features
- **Ingest Anything:** Add text, PDFs, URLs, local files, or entire directories.
- **Smart Categorization:** Automatically suggests and applies tags and categories using AI (configurable).
- **Hybrid Search:** Combines traditional keyword search with semantic vector search for highly relevant results.
- **Multiple AI Providers:** Supports OpenAI, Gemini, Anthropic (and potentially local models) with configurable fallback and retry strategies.
- **Asynchronous Pipeline:** Uses Redis and background workers for efficient processing of embeddings and other AI tasks without blocking ingestion.
- **Pure Go:** Built entirely in Go, ensuring portability and minimal external dependencies (no Python/Java required).
- **Organize Your Way:** Use tags and collections to structure your knowledge base.
- **CLI & API Access:** Interact via a comprehensive command-line interface or integrate with other applications using the REST API.

## Architecture
- CLI: Cobra, API: Gin
- All core validation/logic in service layer
- **ContentService**: filetype detection, extraction (pure Go)
- **CategorizationService**: uses LLM via fallback provider chain
- Embedding abstraction with fallback, retries
- Redis queue worker for async embeddings
- Pluggable vector stores (pgvector)

## Dependencies
- Go 1.20+
- Redis
- PostgreSQL + pgvector
- github.com/ledongthuc/pdf (pure Go PDF)

## Categorization
- Categorizer interface supports LLM-based tagging
- LLM calls via EmbeddingProvider fallback chain
- Categorization auto-applies during add or on demand
- Batch & single categorize via CLI + API

## Embeddings
- FallbackEmbeddingService transparently switches embedding providers
- Providers: OpenAI, Gemini, Anthropic
- Configurable primary + fallback list
- Retries and backoff on failures or rate limits

## Configuration
- `.yaml` config for DB, embedding providers, fallback, chunking
- CLI/environment override flags

## Status & Next Steps (Apr 2025)

**Core Features Implemented:**
- File/URL ingestion & processing
- Hybrid search (keyword + vector)
- AI categorization (stubbed)
- Multi-provider embeddings (stubbed)
- Background job processing

**Pending Tasks / Next Actions:**
1. Implement comprehensive test suite (Target: 85% coverage)
2. Wire up real LLM/Embedding provider API calls (replace stubs)
3. Optimize vector indexing performance
4. Finalize API documentation
5. Performance benchmarking
6. Prepare v1.0 release artifacts

## Commands
- Build: `go build -o mimir ./cmd/mimir` # Correct build command
- Run: `./mimir`
- Test: `go test ./...`
- Lint: `go fmt ./...` ; `go vet ./...`
- List Batches: `./mimir batch list`
- Apply Categories: `./mimir categorize apply <content_id>`
- Batch Suggest Categories: `./mimir categorize batch <id1> <id2> ...`

### Example Usage

**1. Add and Categorize a URL:**
```bash
# Auto-extracts, tags, saves to your knowledge base
mimir add "https://example.com/article" --title "Great Article"
```

**2. Organize Research PDFs into a Collection:**
```bash
# Bulk add, extract text, categorize, and tag
mimir add ./papers/*.pdf --collection ai_research
```

**3. Search by Keyword and Meaning:**
```bash
# Combines semantic and keyword search for relevant hits
mimir search "quantum computing breakthroughs"
```

**4. Auto-Categorize Existing Content:**
```bash
# Use LLMs to retrospectively organize large archives fast
mimir categorize batch --limit 100
```

**5. Use the Developer API (Example):**
```bash
# Integrate hybrid AI search into your apps
curl -X POST http://localhost:8080/api/v1/search -d '{"query": "data privacy laws"}'
```

---
