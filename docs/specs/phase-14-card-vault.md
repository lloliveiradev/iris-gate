# Phase 14: Card vault

**Depends on:** 11. **Closes:** G3 (vault side). **Design:** [`to-be.md`](../design/to-be.md) decisions 7 and 22, sections 8.5 and 10.

## Goal

Build `apps/vault`, a separate Go service that tokenizes test card numbers with envelope encryption under a key manager, so card data never reaches apps/api or apps/web, and the acquirer adapter gets the number only through an audited, mutually authenticated call.

## In scope

1. **Layout:** `apps/vault` (composition) and `libs/vault` (`domain`, `application`, `infra`), added to `go.work` and to `PACKAGES` in the root Makefile, Make targets like the other modules (including `lint` with `configs/go/golangci.yml`), a `vault-isolation` depguard rule in that config, and a Docker Compose service.
2. **Database `vault`** with `vault_migrator` and `vault_app` (same rules as phase 07), tables of to-be section 8.5.
3. **Key manager:** OpenBao in Docker Compose (pinned image), with its files in `infra/docker/openbao/`. An init script there enables Transit and creates the keys: one for data-key wrapping, one for the merchant fingerprint HMAC, one for the risk fingerprint HMAC. The vault authenticates with AppRole (or a token) bound to a **policy that allows only** `datakey`, `decrypt`, `hmac` and `rewrap` on those keys. Never the root token at runtime.
4. **`KeyManager` port** (`GenerateDataKey`, `Decrypt`, `HMAC`, `Rewrap`) with the OpenBao adapter. The domain does not import the OpenBao client.
5. **Tokenize:** `POST /v1/tokens` with a publishable key: test-number allowlist and Luhn, expiry check, CVV format check then discard, AES-256-GCM with a fresh data key and random 96-bit nonce, AAD = token + merchant id, both fingerprints, single-use token expiring in 1 h. Response: `card_token`, `brand`, `last4`, `exp_month`, `exp_year`, `fingerprint`. CORS limited to the web origin, rate limit per key and per IP.
6. **Detokenize:** `POST /internal/v1/detokenize` on an internal listener not published to the host, mTLS required, client certificate must be the apps/api identity. Checks merchant, expiry and single use; writes the `detokenizations` row; marks the token used. Returns the PAN plus `test_scenario` (derived from the allowlist) and `risk_fingerprint`.
7. **Dev PKI:** `make certs` runs a script that writes a local CA and the server and client certificates to `infra/pki/` (the generated files are gitignored; the script and a README are committed). The infra README explains it.
8. **apps/api acquirer adapter:** for vault tokens, detokenize then call the fake acquirer by card number; for magic tokens, keep the phase 11 behavior. Payments stores `fingerprint`, `brand` and `last4` from the vault and sends `risk_fingerprint` and `test_scenario` to the risk service.
9. **Rotation:** `make vault-rotate` rotates the Transit key and runs a rewrap job that rewraps every `wrapped_dek` without decrypting any PAN.

## Out of scope

Checkout pages (phase 15). Saved cards and reusable tokens. An AWS KMS adapter (later; keep the port ready for it).

## Guidelines

- The PAN is `[]byte` from the HTTP decoder to the cipher and is zeroed after use. It is never a `string`, a log field, an error message, a metric label or a span attribute.
- A token belongs to one merchant. Detokenizing with another merchant id fails like an unknown token.
- Reject non-allowlisted numbers **before** any crypto or storage, with an error that does not echo the number.
- Fail closed: if OpenBao is down, tokenization and detokenization return 503; they never fall back to a local key.

## Acceptance criteria

1. Changing one byte of the ciphertext, the nonce, the token or the merchant id in a row makes decryption fail.
2. Two tokenizations of the same card for one merchant have the same `fingerprint`; for two merchants, different `fingerprint` and the same `risk_fingerprint`.
3. A real-looking non-test number is rejected and nothing is stored.
4. Detokenize without a client certificate, or with a certificate that is not apps/api's, fails the TLS handshake or returns 403.
5. A token can be detokenized once; the second call fails; both attempts are audited.
6. After `make vault-rotate`, old tokens still detokenize and every row has the new key version.
7. A log-scan test runs the tokenize-confirm flow with every test card and asserts no card number appears in any service log.
8. The vault database role cannot read `iris` or `antifraud`, and apps/api and apps/anti-fraud roles cannot read `vault`.

## Docs to update

`AGENTS.md` (vault rules, PAN handling), `docs/security.md` (vault, KMS, mTLS now implemented), README (OpenBao, certs, rotation), an ADR for OpenBao Transit as the local key manager.
