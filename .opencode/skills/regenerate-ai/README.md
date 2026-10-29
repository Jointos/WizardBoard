# regenerate-ai — project-local skill

This skill is **project-local**: it lives inside the WizardBoardgame repository
at:

```text
.opencode/skills/regenerate-ai/
```

OpenCode discovers skills under the project’s `.opencode/skills` directory, so
this skill is only advertised and loadable inside this project. Global skills
would live under `~/.config/opencode/skills/`; this one is deliberately not
there. There is no separate scope flag — the directory is the scope.

## Use

- Invoke it explicitly with `@regenerate-ai`, or let OpenCode advertise it from
  its description when a rules change is being discussed.
- The main instructions are in `SKILL.md`.
- Supporting detail is in `references/`.
- The archive helper is `scripts/archive-ruleset.ps1`. It is a **dry run** unless
  `-Apply` is passed. `-RemoveSource` (without `-Apply`) removes old engine
  folders only after verifying the archive zips exist.

## Intent

When the game rules change, this skill tells the agent how to:

1. classify config-only changes (ask before choosing the fast path) versus
   mechanics changes;
2. create **one** new alphabeta baseline like `alphabeta_v2`;
3. create **one** ML engine like `ml_v1` on top of that baseline — more ML
   versions only when explicitly requested;
4. evaluate fairly at the live 1.5 s / depth 6 budget;
5. archive the previous ruleset’s engines, weights, datasets, reports, ratings
   and saves — this step is **mandatory**, and the active `data/` tree must be
   clean before the work is called done;
6. reset the league and update the defaults, catalog and docs.

Nothing is archived until the new engines build, pass tests and are playable.
