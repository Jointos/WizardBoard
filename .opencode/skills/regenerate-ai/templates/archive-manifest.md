# Ruleset archive manifest

- **Archived at:** `<timestamp>`
- **Old ruleset:** `<old-ruleset>`
- **New ruleset:** `<new-ruleset>`
- **Archived by:** `<agent/user>`
- **Rules file hash:** `<sha256 of internal/game/rules.go>`
- **Rules summary:** `<short description of the rules change>`

## Code archive

| Engine folder | ID | Zip | SHA256 |
|---|---|---|---|
| `alphabeta_v1` | `alphabeta-v1` | `alphabeta_v1.zip` | `<hash>` |
| `alphabeta_v2` | `alphabeta-v2` | `alphabeta_v2.zip` | `<hash>` |
| `alphabeta_v3` | `alphabeta-v3` | `alphabeta_v3.zip` | `<hash>` |
| `alphabeta_v4` | `alphabeta-v4` | `alphabeta_v4.zip` | `<hash>` |
| `greedy_v1` | `greedy-v1` | `greedy_v1.zip` | `<hash>` |
| `ml_v1` | `ml-v1` | `ml_v1.zip` | `<hash>` |
| `ml_v2` | `ml-v2` | `ml_v2.zip` | `<hash>` |
| `random_v1` | `random-v1` | `random_v1.zip` | `<hash>` |

## Old pipeline archive

| Command folder | Zip | SHA256 |
|---|---|---|
| `cmd/dataset` | `pipeline/dataset.zip` | `<hash>` |
| `cmd/dataset_v2` | `pipeline/dataset_v2.zip` | `<hash>` |

Each zip contains the complete original folder, including tests, README and
(where applicable) `weights.json`.

## Data archive

| Path | Kind |
|---|---|
| `rankings.json` | Elo table |
| `arena_stats.json` | event stats |
| `models/` | old model/stats JSON |
| `reports/` | old HTML reports |
| `balance/` | old balance JSON/HTML |
| `datasets/` | old training JSONL |
| `saves/` | old game saves |

## Copied source

| Old source | New engine |
|---|---|
| `alphabeta_v2` | `alphabeta_<new-ruleset>_v1` (`alphabeta-<new-ruleset>-v1`) |
| `ml_v1` | `ml_<new-ruleset>_v1` (`ml-<new-ruleset>-v1`) |

The new engine READMEs do not reference old versions; this manifest is the
maintainer record of the copy.

## Cleanup checklist

- [ ] old engine zips verified
- [ ] ML zip contains `weights.json`
- [ ] old data/saves moved
- [ ] catalog imports removed
- [ ] old dataset commands/Makefile targets retired
- [ ] defaults/docs updated
- [ ] new league reset to 1200
- [ ] `go build ./...`, `go test ./...`
- [ ] server/web healthy

## Notes

`<anything the next maintainer needs to know>`
