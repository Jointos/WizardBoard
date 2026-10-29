# Baseline alphabeta (`alphabeta-v2` anatomy)

The full-regeneration path creates **one** new alphabeta baseline from the
`alphabeta_v2` code. The search stays the same; only rule-dependent evaluation,
IDs, and the live/arena filter setting are adapted. Do not create v3/v4-style
variants.

## Files and responsibilities

```text
internal/ai/alphabeta_v2/
├── engine.go       # ID, registration, config, Select/SelectScore
├── search.go       # negamax, alpha-beta, TT, iterative deepening
├── order.go        # candidate filter + move ordering
├── analyze.go      # AI-assist ranking (Analyzer)
├── eval.go         # hand evaluation (the part to adapt)
├── engine_test.go
├── bench_test.go
└── README.md       # Hungarian
```

- Search, TT, iterative deepening, analysis and tests stay structurally the
  same.
- `eval.go`, the ID/name/notes, the README, and the filter configuration are the
  rule-dependent parts.
- The new engine is self-contained. Do not import `alphabeta_v2` or another
  engine package from it.

## Copy and rename

```powershell
Copy-Item -Recurse internal/ai/alphabeta_v2 internal/ai/alphabeta_r2_v1
```

Then:

1. `package alphabeta_v2` → `package alphabeta_r2_v1`
2. `ID = "alphabeta-v2"` → `ID = "alphabeta-r2-v1"`
3. Update `Name`/`Notes` in `ai.Register`.
4. Rewrite `README.md` in Hungarian from the `internal/ai/AGENTS.md` template.
   The “Változások” section says **“Első verzió ebben a szabályrendszerben.”**
   Do not reference old engine versions.
5. Add the blank import to `internal/ai/catalog/catalog.go`.
6. `gofmt -w internal/ai/alphabeta_r2_v1`
7. `go test ./internal/ai/alphabeta_r2_v1`

## v2 hand evaluation (the starting point)

| Term | v2 value | Notes |
|---|---:|---|
| Wizard material | 55 each if 2+ alive, 95 if 1 alive | first wizard is cheap, last is precious |
| Cannon material | 12 | |
| Pawn material | 4 | |
| Buff | +3 per buffed piece | |
| Nerf | −3 per nerfed piece | |
| Pawn advancement | `rank/2` toward the enemy back rank | |
| Aimed cannon | +7 | enemy in a shoot range on the facing ray |
| Wizard danger | cannon 9 / pawn 5 / wizard 2 per threat | subtracted for own wizards, added for enemy threats |
| Tokens | ±2 per token | |
| Terminal | ±1,000,000 | `winScore`; `winScore − ply` in search |

The search itself gets the range/cadence values live from `s.Rules`:
`CannonRange`, `CannonBuffRange`, `PawnForwardCaptureEvery`,
`PawnBuffSideCapture`. The **values above are hard-coded** and must be re-derived
when mechanics change.

## Valuation worksheet (fill before coding)

For every row, write the old value, the new rule, the new value, and the test or
reason. Do not copy blindly.

| Term | Old v2 | New rule / effect | New value | Reason / test |
|---|---:|---|---:|---|
| Wizard value by count | 55 / 95 | | | |
| Cannon material | 12 | | | |
| Pawn material | 4 | | | |
| Buff value | +3 | | | |
| Nerf value | −3 | | | |
| Pawn advancement | `rank/2` | | | |
| Aimed cannon | +7 | | | |
| Wizard danger (cannon/pawn/wizard) | 9 / 5 / 2 | | | |
| Token value | ±2 | | | |
| Terminal score | ±1,000,000 | | | |
| New mechanic (name) | — | | | |

Questions to answer from the rules:

- Is the win condition still wizard elimination? If not, the terminal score and
  wizard value change fundamentally.
- How many wizards exist per side, and what does losing the first vs the last
  cost?
- Do buffs/nerfs stack? Are they permanent? What do they now enable/forbid?
- What are the new cannon ranges, line-of-fire rules and shooting cadence?
- What are the pawn movement, capture and buff-side-capture rules?
- What do tokens do, what is the cap, and what does teleport cost?
- Are there new units, resources, statuses or board changes?

## Live/arena filter parity

`alphabeta_v2` **always** applies `filterCandidates` (see `engine.go` and
`analyze.go`); it has no `DisableFilter` switch. The live/arena default for the
newer engines is **nofilter**.

To compare the new alphabeta and the new ML fairly:

- Port the configurable filter from `alphabeta_v4`:
  ```go
  cands := legal
  if !e.cfg.DisableFilter {
      cands = filterCandidates(&s, legal)
  }
  if len(cands) == 0 {
      cands = legal
  }
  ```
  Do the same in `Analyze`.
- Then run both engines under the same setting. The live/arena default is
  `DisableFilter: true` / `-filter` off.
- If you deliberately keep the filter always on, set the new ML engine to the
  same filter setting and document it; never compare filter-on vs nofilter.

## Tests to adapt/add

- Wizard value: 2 wizards → 55 each; 1 wizard → 95.
- Buffed pawn can capture sideways; unbuffed cannot.
- Nerfed pawn must retreat.
- Cannon buff extends the range; nerfed cannon cannot turn.
- A cannon aimed at an enemy gets +7; a non-aimed one does not.
- Token/teleport value.
- Any new mechanic from the rules.
- Determinism and legal actions (the existing v2 tests already cover this).

## Smoke and sanity

```bash
go test ./internal/ai/alphabeta_r2_v1
go run ./cmd/arena-eval -pairs "alphabeta-r2-v1|alphabeta-r2-v1" -games 10 -time 1500ms -depth 6 -nodes 0 -no-elo -out data/balance/r2_smoke.json
```

Check: no errors, sensible game lengths, no immediate tactical blunders. The
real fair evaluation is in `evaluation.md`.
