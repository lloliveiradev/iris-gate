# Contributing to Iris Gate

Iris Gate is a Go and TypeScript monorepo: `apps/api` (Go), `apps/web` (Next.js), `apps/anti-fraud` (NestJS), and libraries under `libs/`. The task runner is Make. Yarn 4 workspaces install the Node packages. Day-to-day rules for people and coding agents live in [`AGENTS.md`](AGENTS.md). This guide is the contributor path: setup, branches, commits, pull requests, and how to check your work.

## Prerequisites

- Go 1.27 (`go.work` sets `toolchain go1.27.4`, which downloads that toolchain when the installed Go is older)
- Node.js 26 LTS. `package.json` accepts `>=24.11.0`. Enable Corepack so Yarn 4.18.1 is the package manager (`packageManager` in the root `package.json`). On Node 25+, install Corepack first: `npm install -g corepack`
- Docker with Compose

Optional, for hooks and lint: `golangci-lint`, `gofumpt`, `lefthook`, `gitleaks`.

## Setup

From a fresh clone:

```bash
make bootstrap   # yarn install, then the local stack (Postgres, Kafka, Karapace, anti-fraud)
make install-hooks
```

`make bootstrap` runs `yarn install` and `make up`. The first `make up` copies `infra/docker/.env` from `.env.example` when that file is missing. `make install-hooks` runs `lefthook install` and fails if `lefthook` is not on `PATH`.

To install Node dependencies without starting the stack, run `yarn` from the repository root. Go modules resolve through `go.work`. Do not add pnpm, npm, or Turborepo.

`make dev` starts the stack, then the API (`:8080`) and the dashboard (`:3000`). The anti-fraud service is already the Compose container on `:3001`. See [`README.md`](README.md) for ports, the first-run antifraud database, and troubleshooting. `make help` lists the root targets.

## Branch naming

Branch from `main`. Never commit on `main`.

- Phase work uses `phase/<NN>-<slug>`, one pull request per phase, as [`docs/specs/README.md`](docs/specs/README.md) describes.
- Every other branch is `<type>/<short-description>` in lowercase kebab-case. `<type>` is a [Conventional Commits](https://www.conventionalcommits.org/) type: `docs/community-health`, `fix/tenant-check`.

## Commits

Commit subjects are English and follow Conventional Commits. Lefthook rejects anything else:

```text
type(scope): subject
```

`type` is one of `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `build`, `ci`, `perf`, `style`, `revert`. `scope` is optional (`payments`, `identity`, `web`, `repo`). A breaking change may use `type(scope)!: subject`. The subject is at least three characters.

On every commit, Lefthook runs:

- `gofumpt` on staged Go files
- `go vet` on staged files under `apps/api`
- Prettier on staged TypeScript, JavaScript, JSON, YAML, Markdown, and CSS
- gitleaks, when the `gitleaks` binary is installed (the hook skips itself with a warning when it is not)
- the Conventional Commits check above

Do not skip hooks with `--no-verify` unless the owner asks. Format Go with `gofumpt` and TypeScript, JSON, YAML, and Markdown with Prettier (`yarn prettier --write <files>`).

## Push and pull requests

Git over SSH is blocked. Push with HTTPS and open the pull request with the GitHub CLI. Copy [`.github/PULL_REQUEST_TEMPLATE.md`](.github/PULL_REQUEST_TEMPLATE.md), fill every section, and pass that file as the body. The title and the body are English, as is the rest of the repository: code, comments, docs, and commit messages.

```bash
git push https://github.com/lloliveiradev/iris-gate.git docs/community-health
gh pr create --title "Add the contributing guide" --body-file /tmp/pr-body.md
```

The GitHub web form inserts the same template. Never push to `main` (a ruleset protects it) and never force-push. Never merge your own pull request. The owner reviews and merges.

Link the Trello card and the issue when they exist (`Closes #` in the template's Links section).

## Tracking work

Work is tracked on the Trello board **IrisGate**: **Backlog → WIP → Review → Done**. A coding agent stops at Review once its pull request is open and CI is green, and does not move the card to Done. Card titles, descriptions, and comments follow the `.trello.json` locale. That text is board narration, not repository content. A card title has no `[area]` prefix; the area is a label.

The labels, the checklist, who moves a card, and the audit log are specified in [`AGENTS.md`](AGENTS.md) ("Track the work on Trello"). Follow that section rather than a second copy of the board rules.

## Verify before you finish

Run the checks in [`AGENTS.md`](AGENTS.md) that apply to the diff:

- Domain or use-case change: `make test` in that library (and `apps/api` if you touched composition).
- Contract change: `make gen-contracts-check`, then the handler or client that maps the DTO.
- Web change: `make -C apps/web typecheck` and `make -C apps/web test`. If copy or routing changed, check `/en`, `/pt` and `/es`.
- Fraud rule change: `make -C libs/fraud test` and `make -C apps/anti-fraud test`.
- Phase work: every acceptance criterion of the spec, as [`docs/specs/README.md`](docs/specs/README.md) describes.

`make test`, `make lint`, and `make build` run in every package that defines the target. Edit `libs/contracts` specs, then `make gen-contracts`. Do not hand-edit `libs/contracts/generated`.

## Conduct and security

This project uses the [Contributor Covenant](CODE_OF_CONDUCT.md). Report a vulnerability in private, as [SECURITY.md](SECURITY.md) describes. Do not open a public issue for a security report.
