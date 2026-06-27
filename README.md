# 📚 Qalibre

<p align="center">
  <img src="logo.png" alt="Qalibre Logo" width="120" style="border-radius: 8px; box-shadow: 0 4px 10px rgba(0,0,0,0.15);"/>
</p>

<p align="center">
  <strong>A high-performance document database manager built for AI dataset preparation and library curation.</strong>
</p>

<p align="center">
  <a href="https://github.com/mamorett"><img src="https://img.shields.io/badge/Author-@mamorett-blue?style=flat-square&logo=github" alt="Author"/></a>
  <img src="https://img.shields.io/badge/Language-Go%20%2B%20TypeScript-00ADD8?style=flat-square&logo=go" alt="Language"/>
  <img src="https://img.shields.io/badge/License-GPL%20v3-green?style=flat-square" alt="License"/>
</p>

---

## 🎯 Purpose & Goals

The primary goal of Qalibre is to act as a **dataset preparation pipeline** for document-based machine learning models (like LLMs, text extraction, and RAG pipelines). It allows researchers, data engineers, and developers to:

*   **Filter & Catalog:** Query, clean, and organize large document libraries.
*   **Metadata Curation:** Clean, tag, edit, and export schema-compliant metadata.
*   **Format Standardization:** Manage conversion formats (EPUB, PDF, MOBI, etc.) suitable for downstream text extractors.
*   **Data Hygiene:** Apply flexible allowed lists, blocklists, and custom column criteria to filter raw library assets into clean training datasets.

---

## ✨ Features

*   **⚡ High Performance:** Rebuilt with a Go backend for high-throughput queries and sub-millisecond database lookups.
*   **🎨 Premium UI:** Responsive Single Page Application (SPA) frontend styled using Palantir Blueprint v6 with modern dark-mode aesthetics.
*   **📁 Upload Pipeline:** Drag-and-drop file upload with automated metadata parsing (OPF/container mapping) for `.epub` and `.pdf` files.
*   **🔄 Task Runner:** Background queue for serial execution of cleanup tasks, cache management, and database reconnection.
*   **🔒 Security First:** Role-based permissions (Admin, Upload, Download, Viewer), secure session cookies, and Werkzeug-compatible password verifications.
*   **📖 In-Browser Reader:** Native inline PDF/EPUB reading with support for HTTP Byte Range requests.

---

## 🚀 Getting Started

### Running with Docker (Recommended)

To run Qalibre instantly in a self-contained container:

1.  **Build the Image**:
    ```bash
    docker build -t qalibre .
    ```

2.  **Run the Container**:
    ```bash
    docker run -d \
      -p 8083:8083 \
      --name qalibre \
      -v /path/to/calibre/library:/library \
      -v /path/to/config:/app/config \
      qalibre
    ```

> [!NOTE]
> *   `/library` must contain your Calibre library's `metadata.db` and book folders.
> *   `/app/config` is where settings and user sessions (`app.db`) are persisted.

---

### Building from Source

#### 1. Compile the React Frontend SPA
Ensure you have **Node.js 20+** installed:
```bash
cd frontend
npm ci
npm run build
cd ..
```

#### 2. Compile and Launch the Go Server
Ensure you have **Go 1.22+** installed:
```bash
go mod download
go build -o qalibre ./cmd/qalibre
./qalibre
```

---

## ⚙️ Configuration

1.  **Sign In:** Access `http://localhost:8083` and sign in using the default administrator credentials:
    *   **Username:** `admin`
    *   **Password:** `admin123`
2.  **Configure Database:** Navigate to Settings and input the absolute folder path to your Calibre library (e.g. `/library` if mounted in Docker).
3.  **Library Portability:** Qalibre is fully compatible with Calibre's native schema — you can read and write directly to your database with zero migrations.

---

## 👨‍💻 Author & Attribution

Developed and ported to Go by [@mamorett](https://github.com/mamorett).

*Qalibre is inspired by Calibre-Web, but it's a completely new codebase written in Go and TypeScript. It's licensed under the MIT License.*
