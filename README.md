# Enigma

<p align="center">
  <strong>A local-first CLI/TUI hybrid search engine that fuses the web with your personal knowledge.</strong>
</p>

<p align="center">
  <a href="https://github.com/DNahar74/enigma/actions/workflows/ci.yml"><img src="https://github.com/DNahar74/enigma/actions/workflows/ci.yml/badge.svg" alt="CI Status"></a>
  <a href="https://golang.org"><img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go" alt="Go Version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
</p>

---

## Overview

**Enigma** is a fast, local-first search engine and terminal client. It queries remote search providers ([Tavily](https://tavily.com), [Marginalia](https://marginalia-search.com)) and your local notes directory simultaneously, running all results through a transparent, multi-signal ranking pipeline.

Instead of keeping your local knowledge isolated, Enigma actively uses your notes to sharpen web queries and surfaces the most relevant results tailored to what you already know.

```
                  ┌────────────────────────────────────────────────────────┐
                  │                      User Query                        │
                  └──────────────────────────┬─────────────────────────────┘
                                             │
                                   ┌─────────▼─────────┐
                                   │   Local Search    │  (notes/*.md, .txt, .pdf)
                                   └─────────┬─────────┘
                                             │ (Active Query Expansion)
                                   ┌─────────▼─────────┐
                                   │   Remote Search   │  (Tavily, Marginalia fanned out)
                                   └─────────┬─────────┘
                                             │
                     ┌───────────────────────┴───────────────────────┐
                     │              Filter Pipeline                  │
                     │  • Domain Blocklist  • Near-Dedup  • Anti-Slop│
                     └───────────────────────┬───────────────────────┘
                                             │
                     ┌───────────────────────┴───────────────────────┐
                     │              Ranking Pipeline                 │
                     │  • BM25  • Personal Overlap  • Domain Trust   │
                     └───────────────────────┬───────────────────────┘
                                             │
                                   ┌─────────▼─────────┐
                                   │  Renderer / TUI   │  (Lip Gloss & Bubble Tea)
                                   └───────────────────┘
```

---

## Key Features

- 🧠 **Active Query Expansion**: Searches your local knowledge base first, extracts high-value terminology, and enriches web queries automatically.
- 🎨 **Cyberpunk Tabbed TUI**: An interactive terminal UI built with Bubble Tea. Manage multiple tabs, search the web, and read full articles with live filtering and status bars.
- 🖼️ **TrueColor Terminal Graphics**: Web pages aren't just text. Enigma fetches images (including WebP) and renders them inline using TrueColor ANSI half-blocks directly in your terminal.
- 🛡️ **DOM-Depth Resilience**: Automatically flattens deeply nested structural HTML to bypass Go's strict DOM limits, ensuring you can read heavy React apps in the terminal.
- 🔒 **Privacy-First Local Search**: Indexes and searches your local Markdown notes alongside the web. Your personal data never leaves your machine.
- 🎯 **Transparent Multi-Signal Ranking**:
  - **BM25 Relevance**: Per-query Inverse Document Frequency (IDF) scoring across title and snippet.
  - **Personal Vocabulary Overlap**: Boosts web pages that intersect with your personal notes' vocabulary.
  - **Domain Trust**: Configurable boost and penalty weights for developer-focused and authoritative domains.
  - **Explainability**: Inspect score calculations with the `--explain` flag.
- 🛡️ **Noise & Slop Filters**:
  - **Domain Blocklist**: Filter out unwanted domains with full subdomain support.
  - **Near-Duplicate Dedup**: SimHash-based deduplication across multiple search providers.
  - **Anti-Slop Heuristics**: Silently eliminates AI-generated content farms, SEO filler, and spam.
- 💻 **Dual Interfaces**:
  - **Interactive TUI**: Full-featured [Bubble Tea](https://github.com/charmbracelet/bubbletea) interface with interactive navigation and search history.
  - **Scriptable CLI**: Fast, one-shot `enigma search` command with ANSI highlighting and formatted output.
- 🔐 **Secure Credential Storage**: API keys are saved directly into your operating system's native keychain (macOS Keychain, Linux Secret Service / DBus, Windows Credential Manager) via `go-keyring` — never written to disk in plain text.

---

## Installation

### From Source

Ensure you have [Go 1.22+](https://golang.org/dl/) installed:

```sh
# Clone the repository
git clone https://github.com/DNahar74/enigma.git
cd enigma

# Build and install binary
go install ./cmd/enigma
```

Or build locally:

```sh
go build -o bin/enigma ./cmd/enigma
```

---

## Quick Start

### 1. Configure Authentication

Enigma supports API keys stored securely in your OS keychain:

```sh
# Set your Tavily API Key (Required for Tavily search)
enigma auth set-key

# Set your Marginalia API Key (Optional; uses free tier if omitted)
enigma auth set-key --provider marginalia
```

To verify or remove keys:

```sh
enigma auth status
enigma auth delete-key --provider tavily
```

### 2. Search

#### Interactive TUI Mode
Run without arguments to launch the interactive terminal interface:

```sh
enigma
```

- `↑`/`↓` or `j`/`k`: Navigate results
- `Enter` / `o`: Open result URL in browser
- `Esc` / `q`: Exit

#### CLI Command Mode
Run one-shot searches from any shell or script:

```sh
enigma search "Distributed consensus Raft vs Paxos"
```

#### Score Breakdown (`--explain`)
See why results were ranked the way they were:

```sh
enigma search "Memory allocation in Go" --explain
```

---

## Configuration

Enigma automatically generates a default configuration file on first launch:
- **Linux/macOS**: `~/.config/enigma/config.toml`
- **Windows**: `%APPDATA%\enigma\config.toml`

### Example `config.toml`

```toml
[search]
max_results = 20
timeout_seconds = 10
keychain_service = "enigma"
keychain_account = "tavily-api-key"
marginalia_keychain_account = "marginalia-api-key"

[local]
# Path to your personal notes directory (.md, .txt, .pdf supported)
notes_path = "~/notes"

[filter]
blocked_domains = [
    "pinterest.com",
    "quora.com"
]

[ranking]
k1 = 1.2
b = 0.75
personal_boost = 5.0

[trust]
boosted_domains = [
    "github.com",
    "stackoverflow.com",
    "wikipedia.org",
    "go.dev",
    "arxiv.org"
]
penalized_domains = [
    "geeksforgeeks.org",
    "w3schools.com"
]
```

---

## Project Architecture

```
cmd/enigma/               CLI entrypoint and Cobra commands
core/
  config/                 TOML configuration loading, defaults, and validation
  pipeline/               Search pipeline orchestrator (fan-out, filtering, ranking)
  plugin/                 Plugin interfaces (Search, Filter, Rank, Registry)
  query/                  Query tokenization, expansion, and result models
  render/                 Lip Gloss terminal block rendering and highlighting
  secrets/                OS keychain integration (go-keyring)
plugins/
  search_tavily/          Web search via Tavily API
  search_marginalia/      Independent web index via Marginalia API
  search_local/           Local document indexer (.md, .txt, .pdf)
  filter_blocklist/       Subdomain-aware domain blocklist
  filter_antislop/        AI content farm and SEO spam heuristic filter
  filter_dedup/           SimHash near-duplicate result deduplication
  rank_bm25/              Okapi BM25 text relevance ranker
  rank_personal/          Personal notes vocabulary overlap booster
  rank_trust/             Domain reputation booster & penalizer
ui/
  tui/                    Bubble Tea interactive terminal user interface
internal/
  testutil/               Mock plugins and fixtures for testing
```

---

## Development & Testing

Run all unit and concurrency tests:

```sh
go test -race ./...
```

Run linter:

```sh
golangci-lint run
```

Run integration tests (requires network/API keys):

```sh
go test -v -tags=integration ./...
```

---

## Contributing

Contributions are welcome! Please check out [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code style, architecture guidelines, and pull request process.

---

## License

This project is licensed under the [MIT License](LICENSE).
