# Wizard Boardgame

An 8×8 tactical board game with three unit types — **pawns**, **cannons** and
**wizards** — a strict round-robin turn order, and a magic system based on
buff / nerf / cleanse effects.

- **Rules (Hungarian, authoritative):** [`docs/rules.hu.md`](docs/rules.hu.md)
- **Backend:** Go (stdlib only, no external dependencies)
- **Frontend:** TypeScript + Phaser 3 + Vite
- **App language:** English (only the rules document is Hungarian)

## Why this architecture

The rules engine is a **pure, immutable transition function**, designed so a
chess-like machine-learning engine can be built on top of it later:

```go
s := game.New(nil)                 // starting position
legal := s.LegalActions()          // every legal action
next, err := s.Apply(legal[0])     // new state, s is untouched

s2, _ := s.Apply(a)                // same input -> same output, always
s.Hash()                           // 64-bit hash for transposition tables
s.Score()                          // +1 / 0 / -1 from the side to move
s.Encode(nil)                      // []float32 planes for a neural network
```

- `State` is a **value type** with no pointers, slices or maps (except an
  immutable `*Rules`). Copying a state is a memcpy, so parallel search (MCTS,
  negamax, self-play workers) is safe without locks.
- `ApplyInto(dst, action)` is the allocation-free variant.
- The game is **deterministic and perfect-information**, so both negamax and
  MCTS work out of the box.
- Every match keeps its action log, so positions can be replayed bit-exactly.
- Agents plug in through a tiny interface (see `internal/ai`):

```go
type Engine interface {
    Name() string
    Select(s game.State, legal []game.Action) game.Action
}
```

Every engine lives in its own **versioned package** (for example
`internal/ai/alphabeta_v1`) and registers itself with a stable ID. Old versions
are never modified, so their behaviour and their Elo history stay comparable
when a new version is added.

## AI engines (ruleset r2 — wizard swap)

| ID | Package | Notes |
|---|---|---|
| `alphabeta-r2-v1` | `internal/ai/alphabeta_r2_v1` | r2 baseline: negamax + alpha-beta, transposition table, iterative deepening, candidate filtering |
| `ml-r2-v1` | `internal/ai/ml_r2_v1` | 24 side-relative features + 24→64→1 MLP on top of the r2 search (previous default) |
| `ml-r2-v2` | `internal/ai/ml_r2_v2` | 43 features (incl. swap options) + 43→64→1 MLP with score labels — **default engine** |
| `ml-r2-v3` | `internal/ai/ml_r2_v3` | 50 features + 50→96→1 MLP, float32 folded inference, 500-game teacher data |
| `ml-r2-v4` | `internal/ai/ml_r2_v4` | `ml-r2-v3` + quiescence search over captures and shots |

The r2 rules add the **wizard swap**: a wizard may spend its action to exchange
places with any friendly unit on one of the 8 adjacent squares (pawns, cannons
and the other wizard). The swap is free, ends the turn, and buffs/nerfs stay
with the piece.

The r1 engines (`alphabeta-v1..v4`, `ml-v1`, `ml-v2`, `greedy-v1`,
`random-v1`) are **archived** under `internal/ai/archive/r1/` (complete folder
zips, including weights). The r1 data, saves, reports and ratings are under
`data/archive/r1_*/`. The old engine IDs are not registered any more.

`ml-r2-v2` is the default: in the paired 100-game match (1.5 s/move, depth 6,
both engines unfiltered) it beat `ml-r2-v1` **67-29-4 (0.690)**, and the earlier
30-game match beat `alphabeta-r2-v1` **18-6-6 (0.700)**. The current r2 table has
`ml-r2-v2` at **1343** and `ml-r2-v1` at 1110 (160 and 140 games).

### ML engines (`ml-r2-v1`, `ml-r2-v2`)

Both ML engines keep the r2 alpha-beta search and replace the leaf evaluation
with

```
HandEval(position) + clamp(learned model(features), -100, +100)
```

The models are trained only on **r2 alphabeta self-play** (no random/greedy, no
old-rules data). `ml-r2-v1` uses 24 side-relative features (24→64→1, outcome
labels). `ml-r2-v2` adds the two swap-option features and uses dense teacher
scores: 43→64→1, val sign accuracy 79.8%.

```bash
make dataset-r2   # alphabeta-r2-v1 self-play -> data/datasets/r2_ml_v1.jsonl
make train-r2     # fit the 24→64→1 MLP, embed weights.json
make ml-r2-eval   # arena ml-r2-v1 vs alphabeta-r2-v1 (30 games)
```

`ml-r2-v2` is the default; `ml-r2-v1` remains registered for comparison.

```bash
make dataset-r2-v2   # score-labelled r2 alphabeta self-play
make train-r2-v2     # fit the 43→64→1 MLP
make ml-r2-v2-selfplay   # 100 self-play games for a balance report
```

### Balance reports

The arena also records **which side won** — Player A (moves first) or Player B —
plus per-side event stats, endgame survivors, first blood and game length. This
is for reasoning about game balance, not for rating engines or training data.

```bash
make balance            # ml-r2-v2 vs alphabeta-r2-v1, no Elo change
make balance-selfplay   # ml-r2-v2 vs ml-r2-v2: pure first-move / side balance
make arena-report IN=data/balance/balance_mlr2_<stamp>.json OUT=data/balance/balance_mlr2_report.html
```

`-no-elo` records the games in the stats file without touching
`data/rankings.json`; `-paired` uses the same seed with swapped sides to reduce
side-comparison noise; `-stamp` appends `_YYYY-MM-DD_HH-MM-SS` to the output
name. The HTML report is always a **separate** command and is self-contained
(dark theme, collapsible sections, sortable tables, frontend piece icons, 95 %
Wilson intervals). Balance stats are never fed into the ML datasets.

Each stats run also records the top **game-changing moves** (`-best-move`, on by
default): moves by the eventual winner ranked by the reference engine's eval
swing, stored with before/after states and rendered as an SVG board section in
the report just before the Elo table. The action log exists only in the stats
path; live play, self-play and training never collect it. Use
`-best-move=false` to skip it.

The model is embedded in the engine and is used by **both** the live engine and
the AI assist, so the top move and evaluation stay consistent. The candidate
filter (cannon move+turn) is configurable: it is **off by default** for the r2 engines
(matching live play); `-filter` enables it for an ablation.

The engine is **not** machine learning yet; it is a classical search engine
with a hand-written evaluation. The `ai.Engine` interface, the immutable
`game.State`, `Hash()` and `Encode()` are the hooks a learned evaluation or
policy can plug into later.

Live play uses a time budget (`fast` 400 ms, `strong` 1500 ms, `max` 2500 ms).
The arena uses the **same 1.5s live budget with a depth-6 cap**, so tournament
results reflect the actual game strength. Do **not** use a node budget for rating
games; the `-nodes` flag is for debugging only. A full depth-6 search with no
time cap is not practical (one game can exceed 15 minutes).

New engines must follow [`internal/ai/agents.md`](internal/ai/agents.md): one
folder per version, a Hungarian README with a "changes vs previous version"
section, and a fresh `make arena-eval` run.

When the **game rules change**, use the project-local
[`regenerate-ai`](.opencode/skills/regenerate-ai/SKILL.md) skill
(`@regenerate-ai`). It covers the config-only re-evaluation path, the
one-alphabeta + one-ML regeneration, fair evaluation, and archiving the previous
ruleset’s engines, weights, reports, ratings and saves.

## Project layout

```
cmd/server        HTTP + embedded web client (single binary)
cmd/selfplay      automated games between engines
cmd/arena         tournaments + Elo ranking
cmd/arena-eval    new-engine evaluation (depth 6) + Elo and stats JSON
internal/game     pure rules engine (no IO, no globals)
internal/match    in-memory matches, action log, undo, replay, AI turns
internal/api      JSON/HTTP API and DTOs
internal/ai       engine registry + config
internal/ai/agents.md  engine contribution rules
internal/ai/…_v1  versioned engines (r2: alphabeta, ML)
internal/arena    match runner, Elo, rankings and event stats
internal/webui    go:embed of the built Phaser client
web               TypeScript + Phaser client
data/rankings.json  persistent Elo table
data/arena_stats.json  latest cumulative event stats
scripts           helper scripts (kill-port.ps1 for `make kill`)
Makefile          developer shortcuts
docs/rules.hu.md  the game rules (Hungarian)
```

## Running it

### Quick start with make (Windows friendly)

Two terminals:

```bash
make backend     # builds and runs the Go server on http://localhost:8080
make frontend    # installs deps and runs Vite on http://localhost:5173
```

Open <http://localhost:5173>. If a port stays busy (or a `go run` child process
survives Ctrl+C), free it with:

```bash
make kill                # kills whatever listens on :8080
make kill PORT=5173      # any other port
```

The helper prints the process name and PID it killed, and is a no-op when the
port is free. On Windows it is backed by `scripts/kill-port.ps1`
(`Get-NetTCPConnection`, with a `netstat` fallback).

Other targets: `make build` (production bundle + single binary), `make test`,
`make arena` (full tournament), `make ranks`, `make selfplay`, `make clean`.

The client plays synthesized **move, cast, takedown, shoot, teleport, undo and
victory/defeat** sounds using the Web Audio API (no asset files). The speaker
button in the header mutes them; the setting is persisted in localStorage.

The wizard buff teleport glyph is a simplified twin-portal drawing inspired by
the Noun Project “teleport” icon by Lucas Helle; see
[`web/src/assets/ATTRIBUTION.md`](web/src/assets/ATTRIBUTION.md).

> `make` is not part of Windows; install it with `choco install make`,
> `scoop install make` or use the raw commands below.

### Development without make (two terminals)

```bash
# terminal 1 - API on :8080
go run ./cmd/server

# terminal 2 - Vite dev server on :5173 (proxies /api to :8080)
cd web
npm install
npm run dev
```

Open <http://localhost:5173>.

### Production (single binary)

```bash
cd web && npm install && npm run build   # writes internal/webui/dist
cd .. && go build -o bin/wizard-server.exe ./cmd/server
./bin/wizard-server.exe -addr :8080
```

Open <http://localhost:8080>. The client is embedded in the binary, so the
server is fully self-contained.

## Tests and benchmarks

```bash
go test ./...
go test ./internal/game -bench . -benchmem
```

The engine test suite covers the rules, move generation, immutability, replay
determinism, a perft-style node count and a 100-game random fuzz run with
invariant checks. The AI packages add tactical tests (finds a wizard shot),
time/node budget tests, a pending-free-move test, and a full self-play game.

## Self-play and ranking

```bash
# quick head-to-head (depth 6, no node cap)
go run ./cmd/selfplay -games 10 -a alphabeta-r2-v1 -b ml-r2-v2 -depth 6 -time 1500ms

# evaluate every new engine (0 games) against all existing ones
make arena-eval

# full round-robin, e.g. after changing the arena settings
make arena-eval-all

# print the table
make ranks
```

The arena alternates sides (A has the first move, so playing both colours is
required for a fair result), counts draws as half a point, and stores everything
in [`data/rankings.json`](data/rankings.json). Elo starts at 1200 and uses
K=8 per game (batches are 10 games by default). The server exposes the same table
through `GET /api/ai`, which is what the UI's engine picker shows.

[`data/arena_stats.json`](data/arena_stats.json) is the latest cumulative event
summary: `overall`, `aiVsAi.byEngine` and `aiVsAi.byPair`. It counts actions,
buffs/nerfs/cleanses per piece type, and kills both by victim
(`{"pawn": N, "cannon": N, "wizard": N}`) and by the attacker->victim matrix.
See [`internal/ai/agents.md`](internal/ai/agents.md) for details.

## HTTP API

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/matches` | create a match (optional JSON body) |
| `GET` | `/api/matches/{id}` | state + legal actions + history |
| `POST` | `/api/matches/{id}/actions` | play `{"index": n}` from the legal list |
| `POST` | `/api/matches/{id}/ai` | let the configured engine take its turn |
| `POST` | `/api/matches/{id}/undo` | take back to before the last human move |
| `POST` | `/api/matches/{id}/config` | change AI seats / engine / difficulty mid-game |
| `POST` | `/api/matches/{id}/analyze` | rank the top 10 moves (AI assist / balance analysis) |
| `POST` | `/api/matches/{id}/random` | play a random action (dev helper) |
| `POST` | `/api/matches/{id}/save` | write a debug save into `data/saves/` and return its JSON |
| `POST` | `/api/saves/load` | restore a match from a save JSON (new match id) |
| `GET` | `/api/ai` | registered engines with Elo |
| `GET` | `/api/healthz` | health probe |

Debug saves: the header has a floppy icon (save) and a folder icon (load).
Save writes `<date>_<time>-<ms>_<matchid>.json` into `data/saves/` and copies the
JSON to the clipboard; Load reads the clipboard and recreates the match from the
replayable action log (config, rules and seed included). Corrupt files, illegal
actions and hash mismatches are rejected with a 400.

After loading, the **Apply to current** button under the game setup applies the
selected opponent / engine / strength to the running match, so a save created in
one mode (for example human vs human) can be continued in another (for example
human vs AI) without restarting.

## AI assist (balance analysis)

The `✨ AI assist` toggle in the panel shows two things:

- The **engine move** (★ row): the move the engine itself would play, found with
  the same search live play uses (depth 6, the "strong" level). This is the move
  to play if you want to match the AI.
- A **scored list** of the 10 best alternatives with their absolute engine score,
  plus a `N within 5 · M within 15 of K` summary. It is meant for balance work:
  it shows how many genuinely good options a position has, not just the single
  best move.

Every suggestion — the ★ row and all 10 alternatives — is evaluated at the
**same depth** (6, or the configured engine depth). The analysis takes about
3-4 seconds: first it finds the engine move with its own share of the
deterministic node budget, then it ranks the candidates and scores the top 10 at
full depth. Moves more than 15 points behind the best are marked with `≤` (an
upper bound). The summary counts the engine move and the 10 scored alternatives;
because the budget is nodes rather than time, repeated analyses of the same
position return the same ranking. Clicking a row previews the move on the board
without playing it. Both registered engines implement analysis; an unknown
engine ID returns a clear error.

Create body (all fields optional; no body means human vs human):

```json
{ "ai": { "A": false, "B": true }, "engine": "ml-r2-v2", "difficulty": "strong" }
```

After a human action the response has `"pendingAI": true`; the client then calls
`POST /api/matches/{id}/ai`. This keeps the human move visible immediately while
the engine thinks (1.5 s on `strong`).

The server is authoritative: the client never implements rules, it renders the
state and submits indices into the legal-action list it received. That is also
what makes adding online multiplayer later a transport change rather than a
rules change.

## Rules at a glance

- Fixed, symmetric setup; no placement phase. The two middle ranks are empty.
- Turns alternate. Each player has an independent `pawn → cannon → wizard`
  rotation; exhausted unit types are skipped.
- **Pawn:** moves like a king, but may only capture forward on every 3rd own
  action, backward at any time, and sideways only while buffed.
- **Cannon:** shoots exactly 2 squares ahead (through pieces, no line of sight),
  or turns in place, or moves 1 square and optionally turns.
- **Wizard:** skips to gain a token, casts buff / nerf / cleanse only at targets
  exactly on one of its 8 lines (straight or diagonal), stepping onto the empty
  square directly toward the target (adjacent targets cannot be cast on; only
  cleanse needs line of sight), or spends a full 4-token bank to teleport.
- **Win:** a player loses when both of their wizards are dead.

Full details and the exact wording of every rule: [`docs/rules.hu.md`](docs/rules.hu.md).
