# AGENTS.md

## Role

You are working in the Medama monorepo, a self-hosted, cookie-free website analytics project. Produce focused, evidence-grounded changes that preserve the existing product behavior unless the user explicitly asks to change it.

## Repository Map

- `core`: Go server and single-binary application. It owns the OpenAPI contract, SQLite/DuckDB persistence, migrations, the server-rendered dashboard (`core/ui`), and the frozen tracker scripts (`core/client`).

There is no longer a `dashboard` or `tracker` workspace. The React/Remix SPA was removed and replaced by `html/template` + HTMX + Alpine.js + Pico.css rendered from the Go binary; the tracker builds were frozen into `core/client/scripts`. No Node.js, Bun or npm is used to build or run MedamaLite.

For package-specific work, read the nearest package `AGENTS.md` before changing files there. If multiple guides apply, follow all of them; the most specific guide wins when instructions differ.

## Success Criteria

- The task is handled with the smallest correct diff.
- Existing API routes, request/response shapes, deployment behavior, config semantics, licenses, and migration behavior are preserved unless the task requires a change.
- New code follows nearby structure, naming, and test style.
- Generated outputs are changed only through their source-of-truth workflow.
- Relevant validation has been run for each affected package, or the final response explains why it was not practical.

## Constraints

- Work from the repository root unless a package README, mise task, or command says otherwise.
- Inspect the relevant code, mise task config, and package guide before editing. Do not ask the user questions that the repository can answer.
- Root and package `mise.toml` files define shared tool versions and repo workflows.
- Prefer `mise run <package>:<task>` from the root, or `mise run <task>` from the relevant package.
- Prefer existing dependencies and standard library APIs. Add dependencies only when they remove more complexity than they add.
- Keep unrelated formatting, file moves, renames, generated output, lockfile churn, and broad refactors out of the diff.

## Generated Files

Treat these as derived artifacts:

- `core/api/oas_*_gen.go`: generated from `core/openapi.yaml` and ogen config.

Change the source input, run the relevant generator, then review the generated diff. Do not hand-edit generated files.

`core/client/scripts/*.js` are frozen tracker artefacts, not build output. To update them,
rebuild the upstream tracker in a separate checkout and copy `tracker/dist/*.js` in. See
`core/client/README.md`.

## Validation Defaults

- Use package-scoped checks first. Broaden only when the change crosses package boundaries.
- For documentation-only changes, `git diff --check` is enough.
- For behavior changes, run targeted tests for the touched package.
- If imports, generated files, or dependencies change, run the matching generation, formatting, tidy, or lockfile workflow.

## Communication

- For multi-step work, start with a short preamble stating the goal and first action.
- Keep progress updates concise and tied to what changed or what was learned.
- Final responses should summarize changed files, validation run, and any remaining risk or blocker.

## Stop Rules

- Stop and ask when the next action is destructive, affects production/external services, needs missing secrets, or requires a product decision that cannot be inferred from the repo.
- Otherwise, make the most reasonable repo-grounded assumption, proceed, and state that assumption in the final response when it matters.
