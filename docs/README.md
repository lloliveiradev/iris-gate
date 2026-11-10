# Iris Gate documentation

Iris Gate is a portfolio payment gateway: Go API, NestJS anti-fraud service, Next.js dashboard, Kafka and Postgres. It will never run in production or process real money; it exists to show how a market-standard card gateway is built.

## Where to start

| If you want to know | Read |
| --- | --- |
| What the code does today, and what is wrong with it | [`design/as-is.md`](design/as-is.md) |
| Where the project is going, and why | [`design/to-be.md`](design/to-be.md) |
| What is done, what comes next, and in which order | [`roadmap.md`](roadmap.md) |
| The rules and acceptance criteria of a delivery phase | [`specs/`](specs/README.md) |
| How the system is structured and which conventions every change follows | [`architecture.md`](architecture.md) |
| The security baseline and the status of each control | [`security.md`](security.md) |
| Why a structural decision was made | [`adr/`](adr/) |
| Where the project came from | [`legacy-audit.md`](legacy-audit.md) (historical) |

Implementation plans written before each phase live in `plans/`. Rules for editing the repository, for people and agents, are in [`../AGENTS.md`](../AGENTS.md).

## How the documents relate

- **as-is** is a dated snapshot. It is not edited after the fact; a new snapshot replaces it when needed.
- **to-be** is the approved target. It changes only with the project owner's approval.
- **specs** turn the to-be into phases. Each phase updates `AGENTS.md`, `architecture.md` and `security.md` when it changes something they describe.
- **architecture** and **security** describe the system as it stands, and mark what is planned with the phase that delivers it.
- When a document and the code disagree, the code and `libs/contracts` win, and the document is fixed.

## Language

All documentation is in English.
