# Change classification and ask-first gates

Use this file in **Step 0** of `SKILL.md`. The goal is to avoid rebuilding the
whole AI stack for a config tweak, and to avoid silently reusing engine IDs for
new behaviour.

## 1. Read the change

Inspect at least:

- `internal/game/rules.go` — all `Rules` values and defaults;
- `internal/game/actions.go`, `apply.go`, `state.go` — movement, capture,
  shooting, casting, buff/nerf, win condition;
- `internal/ai/ai.go` — `ai.Config` fields that affect search;
- `internal/ai/alphabeta_v2/eval.go` — **hard-coded** valuations;
- `internal/ai/alphabeta_v2/README.md` — documented v2 values.

## 2. Classify

### A. Config-only / rules-value change

Examples:

- `ai.Config`: `MaxTime`, `MaxDepth`, `MaxNodes`, `Seed`, `DisableFilter`;
- UI difficulty / arena settings;
- a `Rules` value the engine already reads dynamically:
  `CannonRange`, `CannonBuffRange`, `TeleportCost`, `MaxTokens`,
  `PawnForwardCaptureEvery`, `PawnBuffSideCapture`, `MaxPly`, …
- a value that changes only tests/docs, not mechanics.

**Do not create a new engine by default.** Ask the user first:

> This looks like a config-only / rules-value change. Choose:
>
> **A (recommended):** keep the current engine IDs, Go files and weights; re-run
> the tests and the evaluation; archive the old ratings/stats only if they are no
> longer comparable. No engine archive.
>
> **B:** regenerate a new ruleset engine anyway
> (`alphabeta-<ruleset>-v1` + `ml-<ruleset>-v1`) and archive the old ruleset.

**Never choose A silently.** If A is chosen:

- do not copy/zip engines;
- keep all Go and weight files;
- run `go test ./...`;
- re-run the arena/evaluation at the live budget if strength settings changed;
- if the old ratings/stats are not comparable, archive them under
  `data/archive/<old-ruleset>_<timestamp>/` and start the same engines at 1200;
- update docs/defaults only if the config is user-facing.

### B. Mechanics / valuation change

Examples:

- a new or removed unit type, board, movement, capture, spell, resource, or win
  condition;
- a changed buff/nerf **meaning**;
- a hard-coded v2 evaluation term becomes invalid:
  - unit values `wizard 55/95 by count`, `cannon 12`, `pawn 4`;
  - `buff +3`, `nerf -3`;
  - pawn advance `rank/2`;
  - aimed cannon `+7`;
  - wizard danger `cannon 9 / pawn 5 / wizard 2`;
  - token `±2`;
  - terminal `±1_000_000`.
- the candidate filter or search semantics must change.

**Full path:** ask/confirm the ruleset label, then follow `SKILL.md` steps 1–6.
The new engines get new IDs and folders; old engines are archived after the new
ones pass.

## 3. Hard-coded evaluation check (A vs B)

The following v2 terms are **not** read from `Rules`; if the rules change any of
them, A may be wrong:

| Term | v2 value | Rule that can invalidate it |
|---|---|---|
| Wizard value by count | 55 each at 2+, 95 at 1 | wizard count / win condition |
| Pawn / cannon material | 4 / 12 | new units, capture rules, costs |
| Buff / nerf value | ±3 | changed spell strength / stacking |
| Pawn advancement | `rank/2` | board size, movement, win condition |
| Aimed cannon | +7 | range, line of fire, shooting rules |
| Wizard danger | cannon 9 / pawn 5 / wizard 2 | new threats, spells, immunity |
| Token value | ±2 | teleport cost, token cap, other uses |
| Terminal | ±1,000,000 | win/loss/draw condition |

If any row is affected, stop and present A and B again with the specific term.

## 4. Ruleset label

- Current ruleset: `r1` by convention (there is no stored ruleset field yet).
- Next full regeneration: `r2`, then `r3`, …
- Ask the user if they use a different label.
- New IDs:
  - `alphabeta-<ruleset>-v1`
  - `ml-<ruleset>-v1`
- Folders:
  - `internal/ai/alphabeta_<ruleset>_v1`
  - `internal/ai/ml_<ruleset>_v1`
- Never reuse old IDs for different mechanics.
- New engine READMEs say “Első verzió ebben a szabályrendszerben”; they do not
  name the old engine versions. The archive manifest records the copied source.

## 5. Ambiguous changes

If it is not clear whether a term is dynamic or hard-coded, ask. Do not guess.
