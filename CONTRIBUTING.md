# Contributing to Enigma

Thank you for your interest in contributing to Enigma! We welcome contributions of all kinds, including bug reports, documentation improvements, feature suggestions, and code changes.

Please take a moment to review this document before submitting your contribution.

---

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

---

## Getting Started

### Prerequisites

- [Go](https://golang.org/dl/) (version 1.22 or higher)
- [Git](https://git-scm.com/)
- [golangci-lint](https://golangci-lint.run/) (recommended for local checks)

### Local Development Setup

1. Fork the repository on GitHub.
2. Clone your fork locally:
   ```sh
   git clone https://github.com/<your-username>/enigma.git
   cd enigma
   ```
3. Verify the setup and run tests:
   ```sh
   go test -race ./...
   ```
4. Build the binary:
   ```sh
   go build -o bin/enigma ./cmd/enigma
   ```

---

## Project Architecture

Enigma is structured around a modular pipeline architecture with clean separation of concerns:

- `core/`: Core abstractions, interfaces, configuration, query parsing, and secrets management.
  - `core/plugin/`: Interfaces (`SearchPlugin`, `FilterPlugin`, `RankPlugin`) and `Registry`.
  - `core/pipeline/`: Asynchronous fan-out, filtering, and scoring coordinator.
  - `core/query/`: Query tokenizer, result types, and term extraction.
  - `core/render/`: Terminal rendering and keyword highlighting with Lip Gloss.
- `plugins/`: Pluggable components implementing `core/plugin` interfaces.
  - `plugins/search_*`: Remote and local search providers.
  - `plugins/filter_*`: Content and domain filters.
  - `plugins/rank_*`: Relevance and trust scoring plugins.
- `ui/tui/`: Interactive Bubble Tea TUI.
- `cmd/enigma/`: Command-line interface and Cobra command trees.

---

## Adding a New Plugin

Enigma's modular design makes extending functionality straightforward.

### Adding a Search Plugin

Implement the `SearchPlugin` interface from `core/plugin/search.go`:

```go
type SearchPlugin interface {
    Name() string
    Search(ctx context.Context, q query.Query) (<-chan query.Result, error)
    Ping(ctx context.Context) error
}
```

Register your plugin in `cmd/enigma/main.go` and include corresponding unit tests in your plugin package.

### Adding a Filter Plugin

Implement the `FilterPlugin` interface from `core/plugin/filter.go`:

```go
type FilterPlugin interface {
    Name() string
    Filter(ctx context.Context, results []query.Result) ([]query.Result, error)
}
```

### Adding a Rank Plugin

Implement the `RankPlugin` interface from `core/plugin/rank.go`:

```go
type RankPlugin interface {
    Name() string
    Score(ctx context.Context, q query.Query, results []query.Result) ([]query.ScoredResult, error)
}
```

---

## Quality Standards & Testing

All contributions must meet our automated quality and test gates before merging:

1. **Formatting**:
   ```sh
   gofmt -s -w .
   ```
2. **Static Analysis & Linting**:
   ```sh
   go vet ./...
   golangci-lint run
   ```
3. **Unit & Concurrency Tests**:
   ```sh
   go test -race ./...
   ```
4. **Module Consistency**:
   ```sh
   go mod tidy
   git diff --exit-code go.mod go.sum
   ```

---

## Commit Guidelines

We use [Conventional Commits](https://www.conventionalcommits.org/) for clean, descriptive commit histories:

- `feat(scope): ...` for new features
- `fix(scope): ...` for bug fixes
- `docs(scope): ...` for documentation changes
- `refactor(scope): ...` for structural code changes
- `test(scope): ...` for test additions or modifications
- `ci(scope): ...` for CI/CD updates

**Example:**
```sh
git commit -m "feat(plugins): add DuckDuckGo search provider plugin"
```

---

## Pull Request Process

1. Create a feature branch from `main`:
   ```sh
   git checkout -b feature/my-new-feature
   ```
2. Make your changes and add unit tests.
3. Ensure all tests and linters pass.
4. Commit using conventional commit format.
5. Push to your fork and open a Pull Request against `main`.
6. Fill out the pull request template with context and verification details.

Thank you for contributing to Enigma!
