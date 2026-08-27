# Enigma — Project Constitution (V0.1)

This document is binding for any agent (human or AI) working on this codebase.
It exists so that "it compiles" is never mistaken for "it's done." If a
request — from the user or from the agent's own judgment — conflicts with
this document, this document wins. Scope changes require an explicit
amendment to this file, not a one-off decision buried in a PR.

---

## 1. What Enigma Is (V0.1)

A **local-first CLI/TUI search tool** that searches the web (via multiple
remote providers) and the user's local notes simultaneously, filters and
ranks both through a transparent multi-signal pipeline, and renders
results either as scriptable one-shot output or in a persistent
interactive session. Browser-based web UI, gRPC/subprocess plugins, and
sync are **out of scope for V0.1** and must not be built, stubbed, or
scaffolded "for later."

**Ship as:** a single static Go binary, `enigma`, invoked as
`enigma search "query"` / `enigma auth set-key`.

### 1.1 In scope for V0.1
- **Search plugins** (multiple, fanned out via the registry):
  - Remote search plugins: Tavily Search API, Marginalia
  - 1 local search plugin: reads `.md`/`.txt`/`.pdf` files under a
    configured notes directory, matches when all query tokens appear in
    the file content
- **Active query expansion**: after local search runs, the top-2
  highest-frequency significant terms (>4 chars, not a stop word, not
  already in the query) from local result titles/snippets are appended to
  the query before remote search runs
- **Filter plugins** (chained, in order): domain blocklist (subdomain-aware)
  → near-duplicate dedup (SimHash/MinHash over title+snippet, drops
  duplicates once multiple remote providers can return the same page) →
  anti-slop heuristic filter (drops results matching 2+ boilerplate
  phrases; local results always pass through unfiltered)
- **Rank plugins** (scores summed): BM25 (per-query IDF, §2.4) + personal
  vocabulary overlap (boosts web results that share vocabulary with local
  notes; scores local results by query-token coverage) + domain trust
  (static boost/penalty list)
- **Score explainability**: an `--explain` flag that prints each rank
  plugin's individual contribution to a result's final score, not just
  the total — this is what makes the "transparent ranking" principle
  actually visible to the user
- **Styled rendering**: a `core/render` package (using `lipgloss`) that
  renders results as bordered blocks with highlighted matched terms;
  shared by both one-shot and interactive modes (§2.7) — plain ANSI
  string concatenation in `main.go` is retired
- **Persistent interactive TUI session** (`bubbletea`): `enigma` with no
  arguments launches an interactive session featuring an indie ASCII art
  startup screen.
- **Tabbed Interface & Terminal Web Reader**: The TUI supports multiple
  tabs (e.g. Search, Details, Web). Users can open search results directly
  in the terminal. The raw HTML is fetched, cleaned (via `goquery`),
  converted to Markdown, and rendered via `glamour`.
- Query pipeline: parse → local search → query expansion → remote search
  (fan-out) → filter (chained) → rank (summed) → sort → render
- Config file (TOML) for blocklist, ranking weights, trust lists, notes
  path, output format — auto-created with defaults (§2.3)
- API key retrieval from OS keychain (per remote provider that needs one)
- Unit tests, integration tests (tagged, opt-in), concurrency + benchmark
  tests

### 1.2 Explicitly out of scope for V0.1
- External browser-based Web UI, gRPC/subprocess plugin transport, sync of any
  kind, result provenance display beyond `--explain`, source-diversity
  ranking, a persistent document index (local search re-scans files per
  query; no on-disk index yet — see §2.6), any provider beyond
  Tavily/Marginalia, workflows/command-suggestion features.
- If a task seems to require one of these, **stop and flag it** rather than
  building a partial version. A half-built connector is dead code.

### 1.3 How scope changes
Nothing above moves into scope by inference. The user updates §1.1 in this
file explicitly before an agent starts building it. This is the mechanism
that prevents scope creep — not agent judgment about what "seems useful."
An agent that discovers scope has already drifted (as happened once here)
must stop and get §1.1 corrected before continuing, not keep building on
an undocumented baseline.

---

## 2. Architecture

### 2.1 Plugin model: Go interfaces, not subprocesses

V0.1 does **not** use gRPC or subprocess plugins, despite the original
design doc. All plugins are Go types compiled into the binary, implementing
interfaces that mirror the documented `.proto` message shapes field-for-field.
This keeps the surface area small now and makes a future `grpcAdapter` that
implements the same interface a mechanical addition later, not a rewrite.

```go
// core/plugin/search.go
type SearchPlugin interface {
    Name() string
    Search(ctx context.Context, q Query) (<-chan Result, error)
    Ping(ctx context.Context) error
}

// core/plugin/filter.go
type FilterPlugin interface {
    Name() string
    Filter(ctx context.Context, results []Result) ([]Result, error)
}

// core/plugin/rank.go
// Batch signature: IDF and other corpus-wide statistics can only be
// computed with the full result set in hand, so RankPlugin scores all
// results for a query in one call rather than one result at a time.
type RankPlugin interface {
    Name() string
    Rank(ctx context.Context, q Query, results []Result) ([]ScoredResult, error)
}

type ScoredResult struct {
    Result Result
    Score  float64
}
```

Rules:
- A plugin type is not added to the registry until it fully implements its
  interface. No partial implementations behind a TODO.
- Plugins must respect `context.Context` cancellation/timeouts — no
  plugin may block past its caller's deadline.
- The registry (`core/plugin/registry.go`) is the *only* place plugins are
  wired in. No plugin may be imported directly by `core/pipeline` or `cmd/`.

### 2.2 Package layout

```
enigma/
├── cmd/enigma/              # main package, CLI entrypoint (cobra), plugin wiring only
├── core/
│   ├── query/                # parsing, Query/Result/ContentSignals types
│   ├── pipeline/              # local→expand→remote→filter→rank→sort orchestration
│   ├── plugin/                # interfaces + registry + composite filter/rank types
│   ├── config/                # TOML loading, validation
│   ├── render/                 # styled result rendering (lipgloss), shared by CLI + TUI
│   └── secrets/                # OS keychain access
├── ui/tui/                    # bubbletea interactive session (§2.7)
├── plugins/
│   ├── search_tavily/
│   ├── search_marginalia/
│   ├── search_local/
│   ├── filter_blocklist/
│   ├── filter_dedup/
│   ├── filter_antislop/
│   ├── rank_bm25/
│   ├── rank_personal/
│   └── rank_trust/
├── internal/testutil/         # shared test fakes (fake plugins, fixtures)
└── go.mod
```

- `internal/` (Go-enforced) holds anything not meant to be imported outside
  this module — this is how we keep the public surface intentional.
- No package may import `cmd/enigma`. Dependencies point inward only.
- **Composite types live in `core/plugin`, not `cmd/enigma`.** A
  `compositeFilter`/`compositeRanker` (or equivalent chaining/summing
  wrapper) is orchestration logic, not CLI wiring — it belongs with the
  interfaces it implements, and it needs its own tests, which is easier
  to enforce for code that isn't sitting in `main.go`. `cmd/enigma` only
  constructs plugins and calls into `core/`.

### 2.3 Config

TOML file at `~/.config/enigma/config.toml`. Schema is validated at startup
with clear errors — an agent must never silently default a missing/invalid
config value without logging why. This applies uniformly: **there is no
per-field exception.** (A prior build silently defaulted invalid `k1`/`b`
ranking parameters instead of erroring, inconsistent with every other
field — that inconsistency is a defect, not a precedent, and must be
corrected to hard-error like the rest of the schema.)

**Bootstrapping rule:** on first run, if the config file does not exist,
`enigma` creates it with sensible defaults and logs
`No config found — created default at ~/.config/enigma/config.toml`. This
applies only to preference data (blocklist, ranking weights, output
format) — it never applies to the API key. If the API key is missing from
the OS keychain, `enigma` hard-errors with the exact command to set it
(e.g. `enigma auth set-key`). Config values get friendly defaults; secrets
never do.

### 2.4 Search provider decision, and ranking

**Tavily over Brave:** Brave's Search API signup required credit card
information even to use the free tier; Tavily did not. That was the
deciding factor — not a quality or feature comparison. If Brave's signup
requirements change, or another provider fits better later, that's a
§1.1 amendment, not a silent swap like the original one.

V0.1 has no document index, so there's no persistent corpus to compute
IDF against. The `rank-bm25` plugin still implements **true BM25**,
including IDF — but computes document frequency (`df`) over the current
query's fetched result set (the N results returned by the search plugin)
instead of a stored corpus. This is standard practice for reranking a
small candidate set and preserves BM25's actual behavior (down-weighting
common terms, up-weighting distinctive ones) rather than degrading to
plain TF-saturation scoring. Do not rename the plugin or drop IDF — if a
future version adds a persistent index, `rank-bm25` should be updated to
use corpus-wide `df` instead of per-query `df`, not replaced.

### 2.5 Local vs. remote search plugin routing

The pipeline must run local search before remote search (to support query
expansion) and needs to distinguish the two kinds of `SearchPlugin`.
**Never route on `Name() == "local"` or any string comparison** — a typo
or renamed plugin silently breaks the pipeline with no compile-time
signal. Instead, local-search capability is a typed marker:

```go
// core/plugin/search.go
type SearchPlugin interface {
    Name() string
    Search(ctx context.Context, q Query) (<-chan Result, error)
    Ping(ctx context.Context) error
}

// LocalSearchPlugin is implemented by search plugins that read the
// user's own data (files, notes) rather than a remote API. The pipeline
// type-asserts for this interface to decide ordering and query-expansion
// eligibility — no name-string comparison.
type LocalSearchPlugin interface {
    SearchPlugin
    IsLocal() bool // always returns true; presence of the method is the marker
}
```

`search_local` implements `LocalSearchPlugin`; `search_tavily` implements
only `SearchPlugin`. The pipeline does `if lp, ok := sp.(LocalSearchPlugin); ok`
to split the registry's search plugins into local/remote groups.

### 2.6 No dead fields

`ContentSignals` (or any exported struct) may only carry fields that at
least one plugin actually populates and at least one consumer (ranking,
filtering, rendering) actually reads. An unpopulated field is dead code
even though `deadcode` won't flag it (struct fields aren't unreferenced
symbols in the way functions are) — this is a known gap in §3.1's
automated enforcement, so it's called out here as a manual review rule.
If a signal isn't populated by anything, remove it until a plugin needs
it; don't leave it as scaffolding.

### 2.7 Rendering and the interactive TUI

Rendering is decoupled from both the pipeline and the CLI entrypoint so
one-shot and interactive modes share it rather than duplicating logic:

```
core/render/     # takes []ScoredResult + Query, produces styled blocks
                 # (lipgloss). No I/O, no terminal control — pure formatting.
ui/tui/          # bubbletea program: owns the event loop, input handling,
                 # keyboard nav state; calls core/render for each result
                 # block and the pipeline (via core/pipeline) for each
                 # search. Only entered when `enigma` runs with no args.
```

Rules:
- `core/render` has no dependency on `bubbletea` — it produces strings/
  `lipgloss` styled blocks that either `cmd/enigma` (one-shot) or `ui/tui`
  (interactive) can print. This is what keeps `--explain` and styled
  output identical in both modes instead of two implementations drifting
  apart.
- `ui/tui` depends on `core/pipeline` and `core/render`; nothing in
  `core/` depends on `ui/tui` (dependencies point inward, per §2.2).
- Keyboard navigation state (selected result index, expanded/collapsed
  snippet) lives in the bubbletea model, not in `core/` — it's
  interaction state, not search state.
- One-shot `enigma search "query"` must keep working and stay scriptable
  (plain stdout, pipeable) — the interactive session is opt-in by running
  `enigma` with no args, not a replacement for the scriptable path.

---

## 3. Non-Negotiable Code Quality Rules

These are enforced by CI (§6), not by agent discretion.

1. **No dead code.** Every exported function, type, and package must be
   reachable from `cmd/enigma` or a test. `deadcode` (golang.org/x/tools/cmd/deadcode)
   runs in CI and fails the build on unreferenced exported symbols outside
   test files.
2. **No unused dependencies.** `go mod tidy -diff` runs in CI; a diff fails
   the build. Every entry in `go.mod` must be justified by actual imports.
3. **No speculative abstractions.** Don't introduce an interface, config
   option, or plugin hook for a feature that isn't in §1.1. YAGNI is a hard
   rule here, not a guideline — speculative generality is exactly how V0.1
   scope creeps into the full design doc.
4. **Errors are handled, not swallowed.** No `_ = err`, no bare `panic()`
   outside `main()` startup validation, no discarded errors from I/O, network,
   or plugin calls. Wrap with `fmt.Errorf("...: %w", err)` for traceability.
5. **No global mutable state.** Config, registry, and clients are
   constructed in `main()` and passed down explicitly. No package-level
   `var` holding live state (constants and pure lookup tables are fine).
6. **Every exported function with a branch has a test** that exercises each
   branch (§4). This is the actual "done" bar — not a coverage percentage.
7. **Lint clean.** `gofmt -l`, `go vet`, and `golangci-lint run` (with the
   config in §6.1) must pass with zero findings before merge.
8. **Readability over cleverness — this codebase is for the user to learn
   from and edit, not just for the agent to produce.** Every exported type
   and function gets a doc comment explaining *why* it exists, not just
   what it does. Prefer explicit, boring code over generics, reflection,
   or dense one-liners — a longer but obvious implementation beats a
   shorter clever one. Functions should be readable in one screen; if a
   function needs scrolling to follow, split it. No abstraction (interface,
   generic type, helper layer) is added unless it's already used in at
   least two places or is required by §2.1 — a single caller doesn't
   justify indirection.

---

## 4. Testing Requirements

### 4.1 Unit tests
- Table-driven, colocated (`_test.go` next to source).
- All plugins tested against the `SearchPlugin`/`FilterPlugin`/`RankPlugin`
  interfaces using fakes from `internal/testutil` — no real network calls
  in the default `go test ./...` run.
- Pipeline orchestration (`core/pipeline`) tested with fake plugins injected
  via the registry, covering: partial plugin failure, empty results,
  context cancellation/timeout, filter removing everything, tie-breaking
  in ranking.

### 4.2 Integration tests
- Build-tagged: `//go:build integration`, excluded from default `go test ./...`.
- Run only via `go test -tags=integration ./...`, manually or in a separate
  scheduled CI job — never blocking the default PR pipeline (avoids quota
  burn and network flakiness gating merges).
- Cover: real Tavily API call shape matches our decode struct, real keychain
  read/write round-trip, config file load against a real file on disk,
  real local-file search against a temp directory with real PDFs/markdown.

### 4.3 Concurrency & load tests
- `go test -race ./...` is part of the default CI run (§6), not optional.
- A dedicated concurrency suite in `core/pipeline` exercises parallel plugin
  fan-out with `-race` against fakes that introduce artificial delay/jitter,
  asserting no data races and correct result merging under concurrency.
- One `go test -bench` benchmark on the filter→rank merge path, tracked
  over time (recorded in CI logs) to catch regressions.
- A separate opt-in throughput test (also `integration`-tagged) exercises
  the Tavily API's rate limits: burst requests past the documented plan
  limit and assert the client backs off/retries correctly rather than
  erroring or silently dropping results.

### 4.4 Definition of done for any change
A change is not done until:
- [ ] New/changed exported behavior has unit tests covering each branch
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run`, `go vet`, `gofmt -l` are clean
- [ ] `go mod tidy -diff` and `deadcode` report nothing new
- [ ] No item from §1.2 was touched
- [ ] If a plugin was added/changed: it fully implements its interface and
      is wired only through the registry

---

## 5. Security Requirements

1. **API keys never touch disk in plaintext.** Retrieved via OS keychain
   (`core/secrets`, using `github.com/zalando/go-keyring` or equivalent).
   Config file must not contain a raw key field — only a keychain reference
   name.
2. **No secrets in logs.** Structured logging must redact anything sourced
   from `core/secrets`. Code review / CI should grep for accidental
   `fmt.Println`/`log.Printf` of key material.
3. **All outbound HTTP calls** (Tavily API) use an `http.Client` with an
   explicit timeout — no default zero-timeout client. TLS verification is
   never disabled.
4. **Input validation**: query strings and config values are validated/size-
   limited before use; no unbounded string concatenation into URLs (use
   `net/url` query encoding, never manual string formatting for request URLs).
5. **Dependency vetting.** `govulncheck ./...` runs in CI; known
   vulnerabilities fail the build. Approved third-party deps: `cobra`
   (CLI), `BurntSushi/toml` (config), `zalando/go-keyring` (keychain),
   `ledongthuc/pdf` (PDF text extraction for local search), `stretchr/testify`
   (test assertions), `golang.org/x/sync` (`errgroup` for concurrent search
   plugin fan-out), `golang.org/x/term` (masked password input),
   `charmbracelet/bubbletea` (interactive TUI event loop), `charmbracelet/lipgloss`
   (styled rendering), `charmbracelet/bubbles` (list/nav components for
   keyboard navigation, if needed rather than hand-rolled) — no further
   dependency added without a one-line justification in the PR
   description.
6. **Rate-limit respect.** The Tavily plugin enforces client-side
   throttling at or below the documented plan limit, independent of
   server-side 429 handling — we don't rely on the API to stop us from
   misbehaving.

---

## 6. CI / Tooling

### 6.1 golangci-lint
Enabled linters at minimum: `govet`, `errcheck`, `staticcheck`, `unused`,
`ineffassign`, `gosec`, `bodyclose`, `noctx`. Config lives at
`.golangci.yml` in repo root — changes to this file require the same
scrutiny as a scope change to §1.

### 6.2 GitHub Actions pipeline (default, on every PR)
1. `go build ./...`
2. `gofmt -l .` (fails on any output)
3. `go vet ./...`
4. `golangci-lint run`
5. `go test -race ./...` (unit + concurrency, integration-tagged tests excluded)
6. `go mod tidy -diff`
7. `deadcode ./...`
8. `govulncheck ./...`

### 6.3 Separate scheduled/manual job
- `go test -tags=integration ./...` — real API/keychain calls, run nightly
  or on manual dispatch, not blocking merges.

### 6.4 Release
- Single static binary per platform (`GOOS`/`GOARCH` matrix: darwin/amd64,
  darwin/arm64, linux/amd64, linux/arm64), built via `go build` with
  `CGO_ENABLED=0` where the keychain library allows it, published as
  GitHub Release artifacts on tag push. No Docker image in V0.1 — it's a
  local CLI, not a service.

---

## 7. Staged Build Order

Built in the sequence below. Each stage must compile, pass its own tests,
and be merged — with the user having read it — before the next stage
starts. No stage may reach ahead into a later stage's work (e.g. don't
write the Brave plugin while "just scaffolding" the registry). This is
what keeps the build reviewable and learnable rather than arriving as one
large diff.

**Stage 0 — Compliance catch-up (do this before any new feature work):**
the build ran ahead of the constitution once already; these close that
gap and must land before stages 10+ below start.
- [x] Fix config validation: `k1`/`b` hard-error on invalid values like
  every other field (§2.3).
- [x] Replace `sp.Name() == "local"` routing with the `LocalSearchPlugin`
  marker interface (§2.5).
- [x] Remove unpopulated `ContentSignals` fields, or populate them if a
  plugin genuinely needs them now (§2.6).
- [x] Move `compositeFilter`/`compositeRanker` from `cmd/enigma/main.go`
  into `core/plugin`, with their own tests (§2.2).
- [x] Add tests for `rank_personal` and for `cmd/enigma` plugin-wiring
  logic (§4.4).
- [x] Stand up the GitHub Actions pipeline exactly as specified in §6.2,
  plus the scheduled integration job in §6.3.
- [x] Confirm/document why Tavily was chosen over Brave: Brave's signup
  required credit card info for the free tier, Tavily's didn't — see §2.4.

1. **Skeleton** — `cmd/enigma` with cobra wired up, `enigma search "query"`
   parses args and prints them back. No plugins, no pipeline. Proves the
   binary builds and the CLI shape is right.
2. **Plugin contracts** — `core/plugin` interfaces (§2.1) plus
   `internal/testutil` fakes for each. No real plugins yet. Proves the
   contracts compile and are testable in isolation.
3. **Pipeline skeleton** — `core/pipeline` wired to the registry using only
   fakes from stage 2, covering the fetch → filter → rank → render flow
   end-to-end with fabricated data. Proves orchestration works before any
   real plugin exists.
4. **Search plugins** — Tavily + local file search, real implementations,
   real integration tests (§4.2). At this point `enigma search` returns
   real unranked, unfiltered results from both sources.
5. **Filter chain** — blocklist + anti-slop, wired in; `enigma search` now
   filters.
6. **Rank plugins** — BM25 (batch, per-query IDF, §2.4), personal
   relevance, domain trust; scores summed. `enigma search` now ranks.
7. **Config** — TOML loading, validation, bootstrapping rule (§2.3); ranking
   weights, trust lists, blocklist, notes path become user-configurable.
8. **Keychain + `enigma auth set-key`** — secrets flow (§5.1), masked input
   via `golang.org/x/term`.
9. **Hardening pass** — full CI suite green (§6.2), `-race` concurrency
   suite and benchmark (§4.3), `govulncheck` clean, integration test suite
   (§4.2) complete, load/rate-limit test (§4.3) run manually and passing.

Stage 9 is the only stage that isn't user-facing new behavior — it's where
the constitution's testing/security bar (§4, §5) gets fully satisfied
rather than partially satisfied stage-by-stage.

**10. Search quality** — Marginalia remote search plugin (fanned out
alongside Tavily); `filter_dedup` near-duplicate filter in the chain
(§1.1) once two providers can return overlapping results. Each merged
with its own unit tests + integration test.

**11. Accuracy & readability** — `--explain` flag exposing per-plugin
score contributions; `core/render` package (§2.7) replacing the ANSI
string concatenation in `main.go`, styled with `lipgloss` (bordered
blocks, highlighted matched terms), used by the existing one-shot
`enigma search` command. Snippet extraction quality improvements land
here too. Output stays plain-text-copyable — styling must not corrupt
piping/redirection (`enigma search "q" > file.txt` should still produce
readable text, ANSI codes stripped when stdout isn't a TTY).

**12. Interactive TUI** — `ui/tui` (§2.7) bubbletea session: `enigma` with
no args launches persistent search, results render via the same
`core/render` package from stage 11, keyboard navigation moves between
results and expands/collapses snippets within a result. `enigma
search "query"` keeps working unchanged as the scriptable path.

Stages 10-12 build in that order — each is independently mergeable and
usable on its own; stage 12 explicitly depends on stage 11's `core/render`
package existing first (§2.7), so it cannot start early.

## 8. Amendment Process

Any change to §1 (scope), §2.1 (plugin transport model), §5 (security
requirements), or §7 (build order) requires the user to explicitly edit
this file first. An agent encountering a request that would require such
a change must stop, name the section that would need to change, and ask —
not proceed and leave the constitution stale.
