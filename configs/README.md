# configs

Tool configuration shared by more than one package. A config that only one app or library uses stays in that package, next to its `package.json` or `go.mod`, because that is where the tool looks for it (`next.config.ts`, `postcss.config.mjs`, `components.json`, `playwright.config.ts`, `vitest.config.ts`, `nest-cli.json`, `jest.config.cjs`, Dockerfiles).

| Path | Used by | What it holds |
| --- | --- | --- |
| [`typescript/tsconfig.base.json`](typescript/tsconfig.base.json) | every TypeScript package, through the two variants below | target, `strict` and the safety flags every package shares |
| [`typescript/tsconfig.node.json`](typescript/tsconfig.node.json) | `apps/anti-fraud`, `libs/fraud` | CommonJS output for NestJS and the libraries it runs |
| [`typescript/tsconfig.bundler.json`](typescript/tsconfig.bundler.json) | `apps/web`, `libs/sdk-web`, `libs/contracts` | ES modules with bundler resolution for Next.js |
| [`eslint/node.mjs`](eslint/node.mjs) | `apps/anti-fraud`, `libs/fraud` | TypeScript rules, plus a rule that stops `domain/` and `application/` from importing NestJS, Prisma or Kafka |
| [`go/golangci.yml`](go/golangci.yml) | every Go module (`make lint`, CI) | golangci-lint v2 with `depguard`: `domain/` and `application/` cannot import transport, drivers or generated DTOs, and contexts cannot import each other |

How packages use them:

- Each `tsconfig.json` starts with `"extends": "../../configs/typescript/tsconfig.<variant>.json"` and adds only its own options (paths, output, JSX, decorators).
- Each Node package's `eslint.config.mjs` is one line: `export { default } from '../../configs/eslint/node.mjs';`. `apps/web` keeps `eslint-config-next`.
- Go module Makefiles run `golangci-lint run --config ../../configs/go/golangci.yml ./...`; CI passes the same path.

Root-level tool files stay at the root because the tools only look there: `package.json`, `yarn.lock`, `.yarnrc.yml`, `go.work`, `.editorconfig`, `.prettierrc.json`, `.prettierignore` (editor plugins read them from the root), `lefthook.yml`, `.gitignore`, `.github/`.

When you add a config here, list it in this table and point every consumer at it in the same change.
