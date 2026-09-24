---
id: T-0315
title: "The entropy validator does not read filenames, hex digests, namespaced class names or a handful of samples as secrets"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-23
started: ""
closed: 2026-09-24
outcome: done
---

# T-0315 · The entropy validator does not read filenames, hex digests, namespaced class names or a handful of samples as secrets

## Goal

Dogfood session 1: 'N/N samples look like secrets' masked as credential, with the fixed $lazyslice$invalid masker, six filename columns (exported reports, logos, media files, screenshots, software releases, webcam shots), four md5 columns and two file-fingerprint columns, three Rails STI 'type' columns holding Module::Class names (every row's class became $lazyslice$invalid, which raises on load), a formatter column, a component_name column, an env-var-name column, an oauth_token_url column, and two columns on 1 and 4 samples. Real secrets (api keys, tokens, password hashes, oauth secrets) were caught correctly and must stay caught. textsig's entropy check should not fire on a value with a file extension, on a fixed-length hex digest (32/40/64), on a dotted or double-colon namespaced identifier, or on a column named type/klass/component_name; and a strong entropy hit needs a minimum sample count (say five) to decide. Pin each shape; measure recall on the truth sets.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 created
- 2026-09-24 closed: done

## Post-mortem

went well: 7268376 (second attempt; the first developer died on the usage limit): filenames, fixed-length hex digests, namespaced class names and env-var names are their own shapes and no longer read as secrets, an entropy hit needs a minimum sample count, and the review's probes (a person's name inside an underscore-joined filename, a dotted handle, an env var carrying a name and a year) each became a stays-masked test; real api keys, tokens and hashes stay caught | went badly: the review found two places where the second net is now looser than the classifier (a '_type' column normalised differently in verify; the exemption reaching JSON leaves), both fail-open, filed for v0.4.0; the torture measurement in the T1 amendment was reasoned about, not re-run | change next time: any classify-side exemption ships with the verify-side twin in the same task, since the second net is the other half of every classifier decision
