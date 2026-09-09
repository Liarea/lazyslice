---
id: T-0104
title: "The credential and online_id name rules miss the spellings an auth schema actually uses"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-08
started: ""
closed: ""
outcome: ""
---

# T-0104 · The credential and online_id name rules miss the spellings an auth schema actually uses

## Goal

docs/TORTURE.md's hand-labelled truth set over supabase-auth (271 columns, 50 labelled personal) measures recall at 0.800, and every one of the ten misses is a column the name rules do not cover: flow_state.auth_code, mfa_challenges.otp_code, mfa_recovery_codes.code_hash, oauth_authorizations.authorization_code, oauth_client_states.code_verifier (the secret half of PKCE), refresh_tokens.parent, scim_users.external_id, identities.provider_id, webauthn_credentials.credential_id and webauthn_credentials.public_key. internal/classify/rules.yml's credential pattern matches tokens?, secrets?, passwords?, api_keys?, private_keys? and their kind, and none of code, auth_code, otp_code, code_hash, code_verifier, credential_id or public_key; online_id matches neither external_id nor provider_id, which are the identifiers an identity provider issues to a person. Seven of the ten are columns the fixture leaves empty, so only the name could have decided them - which is exactly the case a name rule is for. Owed: widen the two patterns and re-measure against the same truth set (the numbers are in docs/TORTURE.md and the labelling rule is stated there, so a change can be scored). Two of the ten deserve a decision rather than a pattern: a WebAuthn public key is a stable per-person identifier and refresh_tokens.parent is a token in a column named after a tree edge.

## Acceptance



## Log

- 2026-09-08 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
