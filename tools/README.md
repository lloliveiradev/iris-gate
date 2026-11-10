# tools

Home of the repository's developer scripts: tasks that support development, testing and demos but are not part of any app or library. There are none yet; the delivery phases add them here.

Expected scripts, by phase:

| Script | Phase | Purpose |
| --- | --- | --- |
| demo seed | 15 | create the demo merchant, users, products and keys (`make demo-seed`) |
| DLQ replay | 13 | re-publish dead-letter messages to their source topic (`make dlq-replay TOPIC=...`) |
| end-to-end runner | 17 | bring the stack up, seed it, run Playwright, tear it down (`make e2e`) |

Rules:

- Scripts live in `tools/scripts/`, one task per script, and every script is reachable through a root `make` target. Nobody should need to remember a script path.
- Shell scripts start with `set -eu` and print what they do; Go or TypeScript tools follow the same conventions as the rest of the code.
- A script never holds a secret. It reads configuration from env or from the `.env` files.
- What a script needs to run inside a container belongs to `infra/` (for example `infra/docker/kafka/create-topics.sh`). Business operations belong to the `irisctl` CLI in `apps/api/cmd/irisctl` (phase 6). Code generation belongs to `libs/contracts`.
