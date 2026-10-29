# AI engine rules

This directory holds one versioned package per engine. These rules keep the
arena comparable, the ratings meaningful and the engines understandable.

## One folder per version

- A new engine **or a new version of an existing engine** gets a **new folder**:
  `internal/ai/<family>_v<N>/`.
- The package registers a **stable ID**: `<family>-v<N>` (for example
  `alphabeta-r2-v1`). Register it from `init()` with `ai.Register`.
- **Never modify a released engine folder.** Copy it to `v<N+1>` and change the
  copy. Old versions stay registered so their Elo history remains comparable.
- Add the blank import to `internal/ai/catalog/catalog.go`; that is the single
  place the server, self-play and arena use to wire all engines.
- Old rankings and match saves must keep working; an engine ID must never be
  reused for different behaviour.

## Required files

Every engine folder contains:

- the engine source (`engine.go` or similar) implementing `ai.Engine`;
- `README.md` **in Hungarian**, written for someone who has never seen the
  engine before;
- at least one test (`engine_test.go`) covering legality, determinism and that
  `Select` does not mutate the state.

### Hungarian README template

```md
# <Név> (<id>)

## Mi ez?
Egy bekezdés: milyen elven működik a motor.

## Hogyan választ lépést?
Lépésről lépésre: milyen lépéseket vizsgál, milyen sorrendben, hol áll meg.

## Értékelés / heurisztikák
Mit és miért pontoz (anyag, pozíció, buff/nerf, tokenek, ...).

## Konfiguráció
Mely `ai.Config` mezőket használja (MaxTime / MaxNodes / MaxDepth / Seed),
és milyen értékekkel fut az élő játékban, az assistban és az arénában.

## Bonyolultság és futásidő
Várható elágazás, mélység, idő / node igény.

## Ismert gyengeségek
Hol hibázik tipikusan, mit nem lát.

## Változások az előző verzióhoz képest
Új verziónál a konkrét változtatások. Az első verziónál: "Első verzió."

## Futtatás és tesztelés
`go test ./internal/ai/<mappa>`, self-play és aréna parancsok.
```

## Quality rules

- `Select` is **pure**: it must not modify the state, and it must return a legal
  action from the supplied list.
- The engine must be **deterministic** for a fixed `ai.Config` (including
  `Seed`). No package-level mutable state, no wall-clock decisions in tests.
- Do not import another engine version's package. Shared helpers belong in
  `internal/ai` or `internal/game`.
- Keep the search self-contained; the game rules stay in `internal/game`.

## Arena and rating evaluation

The arena must play with the **same budget as live play**:

```bash
make arena-eval       # only engines with 0 games, against every existing engine
make arena-eval-all   # full round-robin (use after an arena config change)
```

- Default: **10 games per pair**, sides alternating.
- **Live strong budget**: `-time 1500ms -depth 6 -nodes 0`. A full depth-6
  search with no time cap is not practical (one game can exceed 15 minutes);
  the live game also uses the 1.5s budget with depth 6 as the cap. Do **not**
  use the `-nodes` flag for rating games; it exists for debugging only.
- The heuristic cannon move+turn filter is **off by default** for the r2
  engines (the live default); `-filter` enables it. Compare engines under the
  same setting.
- New engines start at Elo 1200 and are played against **every** existing
  engine. The command updates `data/rankings.json` and merges the event stats
  into `data/arena_stats.json`.
- Use `-reset` for a completely fresh rating (all engines back to 1200, empty
  stats). The command prints each pair's result and duration, so partial
  results are visible while a long tournament runs.

The stats JSON always contains the latest cumulative numbers:

- `sides` — who won by **game side** (`aWin` = Player A, `bWin` = Player B,
  `draw`), independent of which engine sat there. `sides.games` is the number of
  finished games with a recorded side; older files may have fewer than
  `overall.games`.
- `bySide.A` / `bySide.B` — event stats split by the side that acted (actions,
  kills, buffs, nerfs, cleanses). This is the asymmetry check.
- `endgame` — plies, stalemates, survivors and tokens per side at the end, first
  kill / first wizard kill side and ply, game-length buckets.
- `overall` — all evaluated games: actions, buffs/nerfs/cleanses per piece type,
  kills by victim and the attacker -> victim matrix, games and plies;
- `aiVsAi.byEngine` — per engine: Elo, record and the same event stats (mirrors
  the ranking table);
- `aiVsAi.byPair` — head-to-head record, `sides`, `bySide`, `endgame` and event
  stats for each pair;
- `recentRuns` — the last 100 pair runs with duration and `statsOnly` marker;
- `statsOnlyGames` — games recorded by `-no-elo` runs.

Kills use the victim piece type as the key:

```json
"kills": {
  "byVictim": { "pawn": 4, "cannon": 3, "wizard": 1 },
  "byAttacker": {
    "pawn":   { "pawn": 1, "cannon": 2, "wizard": 0 },
    "cannon": { "pawn": 3, "cannon": 1, "wizard": 1 },
    "wizard": { "pawn": 0, "cannon": 0, "wizard": 0 }
  }
}
```

### Balance (statistics-only) runs

The side/endgame numbers exist to reason about **game balance**, not to rate
engines. Use `-no-elo` so `data/rankings.json` is not touched, and a separate
output folder:

```bash
make balance            # ml-r2-v2 vs alphabeta-r2-v1, no Elo change
make balance-selfplay   # ml-r2-v2 vs ml-r2-v2: pure first-move / side balance
```

- `-no-elo` merges the run into the chosen `-out` stats file but never updates
  the ranking table; the global `byEngine` event block is left alone, so the
  pair/overall/side numbers stay honest.
- `-paired` plays games `2k` and `2k+1` with the same seed and swapped sides;
  it reduces side-comparison noise and requires an even `-games`.
- `-stamp` appends `_YYYY-MM-DD_HH-MM-SS` to `-out`.
- Self-play pairs (`a|a`) require `-no-elo`.
- The HTML report is a **separate** command, never generated during the run:
  `go run ./cmd/arena-report -in <stats.json> -out <report.html>` (or
  `make arena-report IN=... OUT=...`).
- These balance measurements are diagnostics. Do not feed them into the ML
  training pipeline; training data comes from the alphabeta engines only.

### Game-changing move

`arena-eval` can keep the replayable action list of every game and rank the
moves played by the eventual winner by how much they improved the winner's
evaluated position. This is **on by default** for statistics generation:

```bash
go run ./cmd/arena-eval ... -best-move            # default: log + analyse
go run ./cmd/arena-eval ... -best-move=false      # no logging, no analysis
```

- Logging is enabled only through `GameConfig.LogGames`; `cmd/arena`,
  `cmd/selfplay`, the live server and the training commands never set it, so
  they have no extra memory or per-move cost.
- The analysis is offline (after the games). It pre-filters with the ml-r2-v2
  hand eval, then re-ranks the top candidates with a reference search
  (`-best-move-engine`, default `ml-r2-v2`; `-best-move-depth`, default 4).
- The final move of a game is skipped, as are skips/passes. Only moves by the
  winner in decisive games are ranked.
- The top `-best-move-top` (default 10) moves are stored in the stats JSON as
  `bestMoves` with their before/after `game.State`, and rendered as the
  "Game-changing move" section (frontend-style SVG boards) in the HTML report,
  just before the Elo table.


The 95% Wilson interval is reported next to every side win rate. With 10 games
per side the interval is very wide: do not call a few percent "imbalance".


## Definition of done for a new engine

- [ ] new folder + stable ID
- [ ] Hungarian `README.md`, including "Változások az előző verzióhoz képest"
- [ ] registered in `internal/ai/catalog/catalog.go`
- [ ] tests pass (`make test`)
- [ ] `make arena-eval` has run; `data/rankings.json` and
      `data/arena_stats.json` are updated
- [ ] ranking table reviewed (`make ranks`)

## Rules changes

When the game rules or rule values change, use the project-local
`regenerate-ai` skill (`@regenerate-ai`, stored in
`.opencode/skills/regenerate-ai/`). It covers:

- the config-only fast path (ask first; keep engine IDs/Go files, re-evaluate);
- the full path with **one** new alphabeta baseline and **one** new ML engine
  (additional ML versions only on explicit request);
- the fair 1.5 s / depth 6 evaluation and promotion;
- archiving the old engines (complete folder zips in
  `internal/ai/archive/<ruleset>/`), the old data (including saves) in
  `data/archive/<ruleset>_<timestamp>/`, and resetting the league.

The archive helper is dry-run by default:
`.opencode/skills/regenerate-ai/scripts/archive-ruleset.ps1`.
