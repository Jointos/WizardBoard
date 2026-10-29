# Fair evaluation and promotion

The new ruleset starts a **fresh league**. Do not compare new engines against
old-rules Elo or stats.

## Promotion check (ML vs alphabeta)

Use the live strong budget and the same filter for both engines:

```bash
go run ./cmd/arena-eval \
  -pairs "ml-<ruleset>-v1|alphabeta-<ruleset>-v1" \
  -games 30 -time 1500ms -depth 6 -nodes 0 -no-elo \
  -out data/balance/<ruleset>_ml_v1_vs_alpha.json
```

- `-time 1500ms -depth 6 -nodes 0` — the live/arena budget.
- `-no-elo` for the promotion check so it does not write the league table;
  the result is a stats-only run.
- Both engines are self-played under the same filter setting. The project default
  is **nofilter**; the new alphabeta and ML must both support/use
  `DisableFilter` consistently.
- At least **30 games**; use paired seeds (or run a second batch) if the result
  is close.
- Look at:
  - score for the ML engine;
  - wins/draws/losses;
  - Player A / Player B balance (the new rules may change side balance);
  - errors, average plies.
- Promote the ML engine only if it is clearly better. If not, keep the new
  alphabeta as default. Record the decision in the archive/manifest or the new
  engine README.

## New league

After the engines are final and the old league is archived:

```bash
go run ./cmd/arena-eval -reset \
  -pairs "alphabeta-<ruleset>-v1|ml-<ruleset>-v1" \
  -games 10 -time 1500ms -depth 6 -nodes 0
```

- Starts every new engine at Elo 1200.
- No old engines, no old ratings/stats.
- 10 games/pair for the default league table; 30+ for a promotion decision.
- `-filter` only if the chosen league setting is filtered; never mix.

## Balance (optional)

```bash
go run ./cmd/arena-eval \
  -pairs "ml-<ruleset>-v1|ml-<ruleset>-v1" \
  -games 100 -time 1500ms -depth 6 -nodes 0 -no-elo \
  -out data/balance/<ruleset>_selfplay.json
```

Then generate the HTML report:

```bash
go run ./cmd/arena-report -in data/balance/<ruleset>_selfplay.json \
  -out data/balance/<ruleset>_selfplay_report.html
```

Balance data is for design/analysis only; it is not training data and does not
affect Elo.

## Verification

- `go build ./...`
- `go test ./...`
- `make build` (web bundle + server)
- start the server; `GET /api/healthz` = 200
- `/api/ai` shows the new default and Elo
- the web engine picker defaults to the winner
- no old engine IDs remain in the default, catalog, Makefile or docs
