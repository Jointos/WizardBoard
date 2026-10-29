# Archive the previous ruleset

Only run this **after** the new engines build, pass tests, and the baseline is
playable. Before applying, show the user exactly what will be zipped/moved and
ask for confirmation.

**The archive step is mandatory.** Do not consider the migration finished while
old-rules engines, ratings, stats, models, reports, balance runs, datasets or
saves are still in the active paths. When the work is reported as complete, the
active tree contains only the new ruleset.

**Preserve the new artifacts before moving folders.** The script moves whole
directories such as `data/datasets` and `data/balance`. Move the new ruleset's
dataset, balance JSON and reports somewhere safe (for example
`data/r2_keep/datasets/` and `data/r2_keep/balance/`) before `-Apply`, then move
them back into the active paths afterwards. Otherwise the fresh r2 artifacts are
archived together with r1.

## Code archive

Every old engine folder is zipped **in its entirety** (Go, tests, README,
`weights.json`) into:

```text
internal/ai/archive/<old-ruleset>/
├── alphabeta_v1.zip
├── alphabeta_v2.zip
├── alphabeta_v3.zip
├── alphabeta_v4.zip
├── greedy_v1.zip
├── ml_v1.zip
├── ml_v2.zip
├── random_v1.zip
├── pipeline/
│   ├── dataset.zip
│   └── dataset_v2.zip
└── MANIFEST.md
```

The zip contains the whole folder, so ML weights and tests travel with the
engine. The old `cmd/dataset` / `cmd/dataset_v2` commands are zipped under
`pipeline/` because they import the archived ML packages and are replaced by the
new ruleset’s `cmd/dataset_<ruleset>`.

After the zips exist and are validated:

- remove the old imports from `internal/ai/catalog/catalog.go`;
- remove/retire old dataset commands that import the old ML packages:
  `cmd/dataset`, `cmd/dataset_v2`;
- remove old Makefile targets that name old IDs
  (`ml-eval`, `ml-v2-eval`, `dataset`, `dataset-v2`, `train`, `train-v2`, …);
- remove old engine folders only after the catalog and commands no longer
  reference them;
- keep the old catalog snippet/Makefile details in the manifest or the git
  history if needed.

## Data archive

Move the old active data (do not leave it in the live paths) into:

```text
data/archive/<old-ruleset>_<timestamp>/
├── rankings.json
├── arena_stats.json
├── models/
├── reports/
├── balance/
├── datasets/
├── saves/
└── MANIFEST.md
```

- `data/models/` includes the old stats/model JSON.
- `data/reports/` and `data/balance/` include the old HTML/JSON reports and
  balance runs.
- `data/datasets/` includes the old training JSONL; it must not be reused.
- **`data/saves/` is archived too.** Old saves are assumed incompatible with the
  new rules; the active `data/saves/` starts empty.
- Leave the existing `data/archive/*` files untouched; add a new ruleset folder.

After moving:

- active `data/rankings.json` and `data/arena_stats.json` are absent/fresh;
- active `data/saves/` is created empty;
- new runs write fresh ratings/stats/reports/datasets for the new rules.

## Reset the league

- All new engines start at Elo 1200.
- Run the new league fresh:
  ```bash
  go run ./cmd/arena-eval -reset -pairs "alphabeta-<ruleset>-v1|ml-<ruleset>-v1" -games 10 -time 1500ms -depth 6 -nodes 0
  ```
  or `make arena-eval` after the new engines are the only ones registered.
- Do not merge old-rules Elo or stats into the new table.

## Script

`scripts/archive-ruleset.ps1` (project root):

```powershell
# dry run
powershell -NoProfile -ExecutionPolicy Bypass -File `
  .opencode/skills/regenerate-ai/scripts/archive-ruleset.ps1 `
  -OldRuleset r1 -NewRuleset r2

# apply: zip engines + move data (asks are handled by the skill)
powershell -NoProfile -ExecutionPolicy Bypass -File `
  .opencode/skills/regenerate-ai/scripts/archive-ruleset.ps1 `
  -OldRuleset r1 -NewRuleset r2 -Apply

# after catalog/default docs are updated (verifies the zips first):
powershell -NoProfile -ExecutionPolicy Bypass -File `
  .opencode/skills/regenerate-ai/scripts/archive-ruleset.ps1 `
  -OldRuleset r1 -NewRuleset r2 -RemoveSource
```

The script:

- auto-detects old engine folders matching `*_vN` under `internal/ai` (excludes
  `catalog` and `archive`);
- zips the old `cmd/dataset` and `cmd/dataset_v2` commands under
  `pipeline/` (they import the archived ML packages);
- writes a manifest with dates, rules hash, zip hashes and data list;
- refuses to overwrite an existing archive unless `-Force`;
- does not edit Go code;
- `-Apply` zips the engines and moves the data;
- `-RemoveSource` **without `-Apply`** removes the old engine/pipeline folders
  only after checking that the archive zips exist; it does not create a second
  data archive.
- `-Apply -RemoveSource` in one run also works, but the normal flow is
  `-Apply` → update catalog/defaults → `-RemoveSource`.

## Cleanup checklist

- [ ] old engine zips exist and are readable
- [ ] each ML zip contains `weights.json`
- [ ] old data (including `data/saves`) is in the ruleset archive
- [ ] `catalog/catalog.go` has no old imports
- [ ] old dataset commands/Makefile targets are removed or rewritten
- [ ] `ai.DefaultEngine` and web default point to the winner
- [ ] README/AGENTS engine table and ruleset notes updated
- [ ] new engine folders build and test
- [ ] new league starts at 1200
- [ ] server/web rebuilt and healthy
- [ ] archive manifest reviewed with the user

## New ruleset IDs

- `alphabeta-<new-ruleset>-v1`
- `ml-<new-ruleset>-v1`
- Additional ML versions (`ml-<new-ruleset>-v2`, …) only when explicitly
  requested.
- Never reuse an archived engine ID for different mechanics.
