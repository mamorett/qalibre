---
name: project-overview
description: Knowledge graph and structural overview of the qalibre project (bookshelf/Calibre Web project)
source: auto-skill
extracted_at: '2026-06-27T00:00:00Z'
---

# Qalibre — Project Overview

## What it is
A modern Go + React bookshelf manager (Calibre-Web replacement) for document-based ML and RAG pipelines. Supports EPUB/PDF/MOBI with drag-and-drop upload, role-based permissions (admin/upload/download/viewer), and a Docker-based deployment.

## Architecture
- **Go backend** (`cmd/qalibre/main.go` → `internal/`): High-performance server with Palantir Blueprint v6 UI, dark mode, secure session cookies, Werkzeug password hashing
- **TypeScript SPA** (`frontend/src/`): React application with React Router for page navigation
- **Calibre-compatible**: Reads Calibre database and native schema (books, authors, comments, shelves)

## Core modules
1. **Auth** — `session.go`, `auth.go`: Session management with random tokens, `GetUserFromContext()`, `HasRole()` for middleware
2. **Routes** — 10 files in `internal/routes/`: REST endpoints for books (CRUD), admin tasks, uploads, downloads, covers
3. **Books** — `epub.go`, `edit.go`: Metadata extraction (OPF parsing), format handling
4. **Server** — `server.go`: Configuration, middleware setup, access control
5. **Tasks** — `tasks.go`, `worker.go`: Background queue (backup, cleanup, cover generation, database reconnect)
6. **DB utils** — `dbutil.go`, `appdb.go`: Database connection, schema init, title sorting, unidecode
7. **Config** — `config.go`: External binary configuration lookup, sanitize settings
8. **Frontend** — Pages for books, edit, auth (login/register), config, admin, reader; components (dialog, layout, pagination, row card)

## Graph highlights (from 335 nodes / 475 edges)
- `RouteManager` (30 edges) is the central hub connecting auth, routes, and server
- `main()` is the wiring point: loads config → creates session manager → starts server
- 27 communities detected with variable cohesion (0.05–0.67)
- 98 weakly-connected isolated nodes suggest documentation gaps
- 21 INFERRED edges around `GetUserFromContext()` and `HasRole()` that may need code verification

## Key decisions from README
- Inspired by Calibre-Web but written in Go/TypeScript
- Supports document-based ML workflows (RAG pipelines)
- 4 role hierarchy: Admin > Upload > Download > Viewer
- In-browser reader with HTTP byte-range requests
- OPF metadata parsing for EPUB/PDF formats

## Files to know
| Path | Purpose |
|------|---------|
| `cmd/qalibre/main.go` | App entry point |
| `internal/server/server.go` | Server setup, middleware chain |
| `internal/auth/session.go` | Session management, token generation |
| `internal/auth/auth.go` | Auth middleware (RequireLogin, RequireAdmin, RequireUpload, RequireViewer) |
| `internal/routes/routes.go` | Route registration, RouteManager |
| `internal/routes/books.go` | Book listing/response helpers |
| `internal/routes/books_write.go` | Book creation/editing/deletion |
| `internal/routes/upload.go` | File upload handling |
| `internal/routes/download.go` | Book downloads |
| `internal/routes/covers.go` | Cover thumbnails |
| `internal/routes/read.go` | In-browser reading |
| `internal/routes/admin.go` | Admin task management |
| `internal/routes/tasks.go` | Task runner endpoints |
| `internal/routes/api_v1.go` | REST API v1 |
| `internal/books/epub.go` | OPF metadata extraction |
| `internal/books/edit.go` | Book metadata editing |
| `internal/tasks/tasks.go` | Task definitions (backup, clean, cover gen) |
| `internal/worker/worker.go` | Background worker |
| `internal/dbutil/dbutil.go` | DB utilities, title sorting |
| `internal/appdb/appdb.go` | Database operations |
| `internal/appdb/schema.go` | Calibre schema definitions |
| `internal/config/config.go` | Configuration handling |
| `internal/csrf/csrf.go` | CSRF middleware |
| `internal/i18n/i18n.go` | Internationalization |
| `internal/logging/logging.go` | Structured logging |
| `internal/mime/mime.go` | MIME type detection |
| `internal/subproc/subproc.go` | subprocess management |
| `frontend/src/App.tsx` | React app shell, AppContext provider |
| `frontend/src/api/client.ts` | API client |
| `frontend/src/context/AppContext.tsx` | Global state context |
| `frontend/src/routes/` | Page components |
| `frontend/src/components/` | Reusable UI components |