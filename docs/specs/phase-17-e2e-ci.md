# Phase 17: End-to-end tests and CI

**Depends on:** 16. **Closes:** G9.

## Goal

Prove the whole system works from a clean checkout, in one command locally and on every pull request in CI.

## In scope

1. **Playwright suite** against the Docker Compose stack, covering:
   - onboarding: self-signup, verification through Mailpit, platform approval, team invite;
   - integration: secret key creation, product and price, demo store checkout with each test card (approved, declined, insufficient funds, blocked, review, timeout);
   - review: approval by a `risk_analyst`, rejection, and expiry with a shortened timeout;
   - manual capture and refunds;
   - webhooks received by a test receiver service that verifies the signature;
   - platform review mode and console metrics.
2. **Stack scripts:** `make e2e`, backed by a script in `tools/scripts/`, brings the stack up, seeds it, runs the suite and tears it down. Short job intervals and timeouts are set through a dedicated env file.
3. **CI jobs** in `.github/workflows/ci.yml`:
   - unit and integration tests for every app and lib;
   - contract drift check and schema compatibility check;
   - cross-tenant suite and route-registry test;
   - Playwright on the Compose stack, with traces uploaded on failure;
   - `gitleaks`, `govulncheck`, `yarn npm audit` (high and above fail);
   - the log-scan test from phase 14.
4. **README:** quickstart rewritten around `make bootstrap`, `make demo-seed`, `make e2e`, with a short tour of what a reviewer can click through.

## Out of scope

Deployment. Load tests (later, with observability).

## Guidelines

- Tests create their own merchants and users; no test depends on another test's data.
- No sleeps: wait on conditions (Playwright locators, polling an API with a deadline).
- CI must finish in under 20 minutes; cache Go modules, Yarn and Playwright browsers.

## Acceptance criteria

1. `make e2e` passes on a clean clone after `make bootstrap`.
2. The CI workflow runs every job above on pull requests to `main` and blocks the merge when one fails.
3. A deliberately broken tenant filter (tested on a throwaway branch) makes the cross-tenant job fail.

## Docs to update

README, `AGENTS.md` (verify-before-finish now includes `make e2e` for cross-cutting changes).
