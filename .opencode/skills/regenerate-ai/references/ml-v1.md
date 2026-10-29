# ML-v1 regeneration (`ml_v1` anatomy)

The full path creates **one** ML engine for the new ruleset from the
`ml_v1` architecture. Additional ML versions are created **only when the user
explicitly asks for one**.

## What ml_v1 is

- A self-contained copy of the base engine's search (originally v4's negamax +
  alpha-beta + TT + iterative deepening).
- Leaf evaluation:
  ```text
  eval = HandEval(position) + clamp(model(features), -100, +100)
  ```
- 24 side-relative features, 32-hidden MLP or linear ridge, weights embedded via
  `go:embed weights.json`.
- Candidate filter configurable; live/arena default nofilter.
- The same model is used by the engine and the AI assist.
- Default ml_v1 training label: **outcome** (+1/0/−1 for the side to move);
  optional score labels are a later ML-v2-style step.

## New engine

Create `internal/ai/ml_<ruleset>_v1`, ID `ml-<ruleset>-v1`.

1. Copy the old `ml_v1` folder as the starting point, then rename the package,
   ID, `Name`/`Notes`.
2. Replace its search/core with the **new alphabeta baseline**’s code so the new
   ML is self-contained and rule-compatible. Keep the `ml_v1` layer:
   - `features.go`
   - `model.go` (embed weights)
   - `evaluate`/`HandEval`
   - `analyze.go`
3. Update `HandEval`/`evaluate` to the new alphabeta’s evaluation and the new
   rules. Do not import an old engine package; copy the needed code.
4. Update the feature vector for new mechanics:
   - unit counts/buffs/nerfs per type;
   - tokens/teleport;
   - phase;
   - pawn advancement;
   - aimed cannons;
   - cannon mobility;
   - wizard danger;
   - piece count;
   - hand eval / 100;
   - ply / MaxPly;
   - pending;
   - aim target types.
   Add/remove feature dimensions when the rules add/remove states.
5. Keep the same live/arena filter setting as the new alphabeta. The default is
   nofilter; pass `DisableFilter` through consistently.
6. Add the catalog import and a Hungarian README:
   - “Első verzió ebben a szabályrendszerben.”
   - no references to old ML versions.
7. `go test ./internal/ai/ml_<ruleset>_v1`.

## Dataset

Copy/adapt `cmd/dataset` → `cmd/dataset_<ruleset>`:

- import the new `ml_<ruleset>_v1` feature package;
- pair list: **only the new alphabeta**, self-play (no random/greedy, no old
  IDs);
- fresh output, e.g. `data/datasets/<ruleset>_ml_v1.jsonl`;
- default label `outcome`, depth 6, 500 ms per move (ml_v1-style), several
  hundred to a few thousand games;
- do not mix old-rules data or old datasets.

The old `cmd/dataset` and `cmd/dataset_v2` import the old ML packages. After the
new command works, retire/archive them and remove their Makefile targets.

## Train and embed

```bash
go run ./cmd/train \
  -in data/datasets/<ruleset>_ml_v1.jsonl \
  -out internal/ai/ml_<ruleset>_v1/weights.json \
  -model mlp -target outcome
```

- Use the existing `cmd/train` (game-level train/val/test split,
  standardization, Adam + early stopping).
- Keep the weights file embedded with `go:embed`.
- An empty/placeholder weights file means the engine plays the hand eval; do not
  ship that as the final ML engine.

## Fair evaluation and promotion

```bash
go run ./cmd/arena-eval \
  -pairs "ml-<ruleset>-v1|alphabeta-<ruleset>-v1" \
  -games 30 -time 1500ms -depth 6 -nodes 0 -no-elo \
  -out data/balance/<ruleset>_ml_v1_vs_alpha.json
```

- Same budget and filter for both engines: **1.5 s / depth 6 / nofilter**.
- At least 30 games; check the score and the side balance.
- If the ML engine is not clearly better, keep the alphabeta as default.
- If promoted:
  - `ai.DefaultEngine = "ml-<ruleset>-v1"`;
  - web default + fallback list;
  - README/AGENTS engine table;
  - start the new league at Elo 1200.
- Optionally run a rated league after the promotion check; do not mix old-rules
  Elo.

## Additional ML versions

Only if the user explicitly asks, **and only as part of a ruleset regeneration
that is already in progress**. When the rules are unchanged, a new ML version is
ordinary AI work: it does not need this workflow, new ruleset IDs or an archive.

- Use the next version: `ml-<ruleset>-v2`, folder
  `internal/ai/ml_<ruleset>_v2`.
- Same ruleset, same new alphabeta teacher, same fair evaluation.
- Keep the existing ML version registered unless the user asks to archive it.
- Possible improvements: score labels (ML-v2 style), more features, paired
  seeds, more games. Do **not** create these proactively.
- The old `ml_v2` code/weights are archived with the previous ruleset; do not
  reuse its 41-feature model across rules without retraining.

## Makefile targets

Add the ruleset-specific targets:

```make
dataset-<ruleset>:
	go run ./cmd/dataset_<ruleset> -games 500 -time 500ms -depth 6 -workers 8 -seed 1 -out data/datasets/<ruleset>_ml_v1.jsonl

train-<ruleset>:
	go run ./cmd/train -in data/datasets/<ruleset>_ml_v1.jsonl -out internal/ai/ml_<ruleset>_v1/weights.json -model mlp -target outcome

ml-<ruleset>-eval:
	go run ./cmd/arena-eval -pairs "ml-<ruleset>-v1|alphabeta-<ruleset>-v1" -games 30 -time 1500ms -depth 6 -nodes 0
```

Adjust the game count and worker count to the machine; the training data must
come from the new rules only.
