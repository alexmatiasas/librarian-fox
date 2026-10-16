# Tasks (Plan/Build/Review loop)

Flow:
1. Plan (read-only): write docs/tasks/NNN-name.md with goal, AC, scope, out of scope, exact steps to verify manually.
2. Build (worktree): small steps, run `just check` after each step and commit. If fails, no commit.
3. Review (read-only subagent, free model): compare diff vs task file; report AC met/missed and out of scope.
4. Verify: read task summary, run manual verification steps, review diffs commit-by-commit. Merge to develop. Task marked done.
