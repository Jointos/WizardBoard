---
name: regenerate-ai
description: Use only when the game rules or rules values change (mechanics, movement, capture, spells, buff/nerf, win condition) and the AI must be re-evaluated or regenerated, including archiving the old ruleset. Do not use for routine AI work with unchanged rules — extra arena runs, balance reports, or an additional ML version.
---

# Regenerate AI after a rules change

This is a **project-local skill for the WizardBoardgame repository**. It lives in
`.opencode/skills/regenerate-ai/`; global skills would live under
`~/.config/opencode/skills/`. There is no extra scope flag: the directory is the
scope.

## Scope

**Rules changes only.** If the rules are unchanged, skip this skill: routine
arena runs, balance reports, evaluation reruns, engine tweaks or an additional
ML version are normal AI work and do not need the archive/regeneration workflow.

Read the reference files for the branch you take. Do not skip the **ask-first**
gates.

## Hard rules

- **Never reuse an engine ID for different mechanics.** A full rules change gets
  a new ruleset namespace: `alphabeta-<ruleset>-v1`, `ml-<ruleset>-v1`.
- **Do not archive, zip or delete anything before the new engines compile, pass
  tests and the baseline is playable.**
- **Ask before destructive actions.** `scripts/archive-ruleset.ps1` is a
  dry-run unless `-Apply` is passed.
- **No old-rules training data.** Old datasets are archived; the ML engine is
  trained from the new rules only.
- **No random/greedy in training.**
- **One alphabeta + one ML by default.** Within a ruleset regeneration,
  additional ML versions may be created only when the user explicitly asks for
  one. Never create `ml-v2`, `ml-v3`, … proactively. (With unchanged rules a new
  ML version is ordinary work and is out of scope here.)
- New engine READMEs are in **Hungarian** and follow
  `internal/ai/AGENTS.md`. They do **not** reference old engine versions; for a
  new ruleset write “Első verzió ebben a szabályrendszerben.” The archive
  manifest records what was copied for maintainers.

## Step 0 — classify the change, then ask

Read `references/change-classification.md`.

### 0A. Config-only or rules-value change

This means AI config (time/depth/filter/seed), UI/difficulty, or a `Rules` value
that the engine reads dynamically (cannon range, teleport cost, pawn cadence, …).

**Stop and ask the user**:

> This looks like a config/rules-value change. Choose:
> **A (recommended):** keep the current engine IDs, Go files and weights;
> re-run the evaluation; archive old ratings/stats only if they are no longer
> comparable; no engine archive.
> **B:** regenerate a new ruleset engine anyway
> (`alphabeta-<ruleset>-v1` + `ml-<ruleset>-v1`) and archive the old ruleset.

- **Never choose A silently.**
- If A is chosen and a **hard-coded** evaluation term is now wrong (unit values,
  buff/nerf ±3, pawn advance, wizard danger weights, wizard value by count, …),
  report it and ask before touching `eval.go`. That may turn the work into B.
- If B is chosen, continue with the full path below.

### 0B. Mechanics / valuation change

Examples: new/removed mechanics, units, board, win condition, buff/nerf meaning,
a hard-coded evaluation term becoming invalid.

- Ask/confirm the ruleset label. The current ruleset is `r1` by convention; the
  next is `r2`, then `r3`, etc.
- Use the full path (steps 1–6).

If the classification is unclear, **ask**; do not guess.

## Step 1 — reference and copy (before archiving)

Read `references/baseline-v2.md` and `references/ml-v1.md`.

- Read the old `internal/ai/alphabeta_v2` (search, `eval.go`, README) and
  `internal/ai/ml_v1` (features, model, README).
- Fill the **valuation worksheet** from `references/baseline-v2.md`: for every
  v2 term, record the old value, the new rule, the new value and why.
- Copy only the code needed into the new, self-contained engine folders. Do not
  import an old engine package from a new engine.
- **Do not archive yet.**

## Step 2 — exactly one alphabeta

Create `internal/ai/alphabeta_<ruleset>_v1` with ID
`alphabeta-<ruleset>-v1`, copied from `alphabeta_v2`.

- Keep the **search unchanged** (negamax, alpha-beta, transposition table,
  iterative deepening, analysis). Adapt only rule-dependent evaluation and the
  ID/package/README.
- No v3/v4-style variants. One alphabeta baseline.
- Port the configurable candidate filter so the new league can compare both
  engines under the same setting (see `baseline-v2.md`).
- Add/adapt tactical tests; run `go test`; run a 10-game self-play smoke test.
- Register it in `internal/ai/catalog/catalog.go`.
- Hungarian README with “Első verzió ebben a szabályrendszerben.”

## Step 3 — exactly one ML

Create `internal/ai/ml_<ruleset>_v1` with ID `ml-<ruleset>-v1`, copied from
`ml_v1`.

- The base/teacher is the **new alphabeta**, not the old v4. Keep the ML package
  self-contained: copy the new alphabeta search/eval into it and keep the
  ml_v1 feature/model layer.
- Update `HandEval`, `evaluate` and the feature vector for the new rules.
- Copy/adapt `cmd/dataset` to `cmd/dataset_<ruleset>`:
  - feature import = the new ML package;
  - pair list = the new alphabeta only (self-play), no random/greedy, no old
    IDs;
  - fresh output path under `data/datasets/`.
- Generate fresh data, train with `cmd/train`, and embed `weights.json`.
- Evaluate fairly at **1.5 s / depth 6 / no filter**, 30+ games. If it beats
  the baseline, make it the default; otherwise the alphabeta stays default.
- **Additional ML versions** (`ml-<ruleset>-v2`, …) only when the user
  explicitly asks for one. Use the same ruleset, teacher and fair evaluation;
  keep the existing version unless the user asks to archive it.
- Add Makefile targets: `dataset-<ruleset>`, `train-<ruleset>`,
  `ml-<ruleset>-eval`.

## Step 4 — defaults, docs and wiring

- `internal/ai/catalog/catalog.go`: add the new engines.
- `internal/ai/ai.go`: `DefaultEngine` = the fair-evaluation winner.
- `web/src/app.ts`: default engine + fallback AI list entry.
- Root `README.md` and `internal/ai/AGENTS.md`: engine table, default, ruleset
  note, Makefile targets.
- Remove/replace references to old engine IDs in docs, examples and defaults.
- Do **not** add old-version references to the new engine READMEs.

## Step 5 — archive the old ruleset (ask first)

Read `references/archive-all.md`.

**Archiving is a required step, not cleanup at the end of the week.** Do not
report the regeneration as done while old-rules engines, ratings, stats, models,
reports, balance runs, datasets or saves are still in the active paths. "The new
engines work" is not the finish line; the active tree must contain only the new
ruleset.

0. **Preserve the new ruleset's artifacts first.** The archiver moves whole
   folders (`data/datasets`, `data/balance`, …). Move the new run's dataset and
   balance output to a temporary place (e.g. `data/r2_keep/datasets/`,
   `data/r2_keep/balance/`) before `-Apply`, then restore them afterwards.
   Otherwise the fresh r2 dataset/report is archived with r1.
1. Run the dry run (no changes):
   ```powershell
   powershell -NoProfile -ExecutionPolicy Bypass -File `
     .opencode/skills/regenerate-ai/scripts/archive-ruleset.ps1 `
     -OldRuleset r1 -NewRuleset r2
   ```
2. Review the printed engine folders and data paths with the user.
3. Apply the archive:
   ```powershell
   powershell -NoProfile -ExecutionPolicy Bypass -File `
     .opencode/skills/regenerate-ai/scripts/archive-ruleset.ps1 `
     -OldRuleset r1 -NewRuleset r2 -Apply
   ```
   This zips every old engine folder **in its entirety** into
   `internal/ai/archive/r1/` and moves old data (including `data/saves`) into
   `data/archive/r1_<timestamp>/`.
4. Update the catalog/docs/defaults, then remove the old engine folders
   (the script prints the exact paths; run the script again with
   `-RemoveSource`, **without** `-Apply`, to remove them after the archive is
   verified).
5. Remove/retire old dataset commands and Makefile targets.
6. Reset the active league:
   - no old `data/rankings.json` / `data/arena_stats.json`;
   - run the new engines fresh at Elo 1200 (`arena-eval -reset` or empty files).
7. Create empty active `data/saves/`, `data/datasets/`, etc. as needed.

## Step 6 — verify

- `go build ./...`
- `go test ./...`
- `make arena-eval-all` (or the new ruleset’s league command)
- `make build`; start the server; `GET /api/healthz` = 200.
- Confirm the new default in `/api/ai`, the web UI and the README.
- Confirm the archive contains the engine zips (including `weights.json`),
  data JSON/HTML, and `data/saves`, with a manifest.
- **Confirm the active data tree is clean:** no old `data/rankings.json` /
  `data/arena_stats.json`, no old `data/models/`, `data/reports/`,
  `data/balance/` or `data/datasets/` entries, and `data/saves/` is empty.
  Everything old must be under `data/archive/<old-ruleset>_<timestamp>/`.
- Confirm no old engine IDs remain in the catalog, defaults, web fallback,
  Makefile, README or `AGENTS.md` (an explicit "r1 is archived" note is fine).

## Reference files

- `references/change-classification.md` — config fast path vs full regeneration,
  ask-first templates.
- `references/baseline-v2.md` — v2 anatomy, valuation worksheet, copy/adapt
  checklist.
- `references/ml-v1.md` — ml_v1 architecture, dataset/train/evaluate, additional
  ML versions.
- `references/archive-all.md` — code/data/saves archive layout, cleanup checklist.
- `references/evaluation.md` — fair arena and promotion rules.
- `templates/archive-manifest.md` — manifest filled by the archive script.
- `scripts/archive-ruleset.ps1` — dry-run archiver.
