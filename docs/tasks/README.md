# Tasks (Plan/Build/Review loop)

Flow:
1. Plan (read-only): write `docs/tasks/NNN-name.md` — goal, acceptance criteria, scope,
   out of scope, exact manual verification steps. User approves before code.
2. Build (worktree): small steps; run `just check` after each step and commit. Failed check = no commit.
3. Review (read-only subagent, free model): compare diff against the task file; report criteria
   met/missed and anything out of scope.
4. Verify: read the summary, run the manual steps, review diffs commit-by-commit.
5. Merge to develop; mark the task file done.

## Status

| # | Task | Status |
|---|---|---|
| 000 | Fase 0 — OCR validation | DONE 2026-10-19 |
| 001 | Fase 1b — Real inventory scan | Planned |
| 002 | Real-book validation program (stages A–D) | Planned, awaiting approval |
| — | Fase 2 — LLM classification | Backlog (starts after 002-A) |
| — | Fase 3 — Restore/OCR pipeline | Backlog (002-B feeds it) |
| — | Fase 4 — Apply/rename | Backlog (002-D feeds it) |
| — | Fase 5 — Kavita/OPDS | Backlog |

Task file is written when the stage starts, not before.
