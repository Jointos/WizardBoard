# Ruleset archive manifest

- **Archived at:** 2026-10-28_11-22-57
- **Old ruleset:** r1
- **New ruleset:** r2
- **Rules file hash:** `6ef395828e074e800f7440ff934688ba18a74b44ee00cbc79de3cdde1719ba4b`

## Code archive

| Engine folder | Zip | SHA256 |
|---|---|---|
| `alphabeta_v1` | `alphabeta_v1.zip` | `ba7b897f6ecaddc95951bc12738380eee2f8938222ef27d7ea7e33882cb64cb4` |
| `alphabeta_v2` | `alphabeta_v2.zip` | `3955ec08365ee3e800ca9851a317a21fbc4a5622a6feb25aa6450f3cf3833343` |
| `alphabeta_v3` | `alphabeta_v3.zip` | `93f9a57261eb2f61cd41f41a44defec008e3e853086cbbaefbad9b4ec4462184` |
| `alphabeta_v4` | `alphabeta_v4.zip` | `799189271938d599acdeb2544ab146eec5a44417b6d0c8b86d7af07b55c17aff` |
| `greedy_v1` | `greedy_v1.zip` | `4141144e578fc7349e70da4b70f2775a0ef39d78aef5f5ef080e824eb1995f42` |
| `ml_v1` | `ml_v1.zip` | `209fae99236abdf484bd540ac42770f2efa561534d5820763c8dedfb17c7bcc1` |
| `ml_v2` | `ml_v2.zip` | `7d4db696724d945b19f4fdd0529653bd49a571aa7850cab8b17e5d1ca0d12c4b` |
| `random_v1` | `random_v1.zip` | `58c76a61790e9ed276917a6c48f14112acbb0991717fc0e6807db0811cafcfc7` |

Each zip contains the whole original engine folder (Go, tests, README and
`weights.json` where applicable).

## Old pipeline archive

| Command folder | Zip | SHA256 |
|---|---|---|
| `dataset` | `dataset.zip` | `d8c9dfe70514a9d36832b03e0a41b121bf9ccc54feec1cbb244b1e88d7834942` |
| `dataset_v2` | `dataset_v2.zip` | `e519a97b8f6a535e6107f8ba46c393c30bbc05538df89932a4806f24f4c14b90` |

These are the old dataset commands that imported the archived ML packages. Keep
them for reference; the new ruleset gets a fresh `cmd/dataset_<ruleset>`.

## Data archive

| Path | Status |
|---|---|
| `data/rankings.json` | archived |
| `data/arena_stats.json` | archived |
| `data/models` | archived |
| `data/balance` | archived |
| `data/datasets` | archived |
| `data/saves` | archived |

## New engines expected

- `alphabeta-r2-v1`
- `ml-r2-v1` (additional ML versions only on explicit request)

## Cleanup checklist

- [ ] old engine zips verified
- [ ] ML zip contains weights.json
- [ ] old pipeline zips verified
- [ ] old data/saves moved
- [ ] catalog imports removed
- [ ] old dataset commands/Makefile targets retired
- [ ] defaults/docs updated
- [ ] new league reset to 1200
- [ ] go build ./...
- [ ] go test ./...
- [ ] server/web healthy