---
id: T-0232
title: "Curate --unmask flags for gitlab, odoo, discourse and supabase-auth under T-0198's broadened DDL-literal rule"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-16
started: ""
closed: ""
outcome: ""
---

# T-0232 · Curate --unmask flags for gitlab, odoo, discourse and supabase-auth under T-0198's broadened DDL-literal rule

## Goal

T-0198 (docs/TORTURE.md's own new section) landed a masked column's own CHECK/generated-expression/index-predicate/non-rewritable-DEFAULT refusing on any literal, not only a strong-validator hit, and measured the cost on make torture: five of the ten real schemas newly refuse after the empty-collection and similar_escape fixes, and metabase's two flags are already landed. The other four still fail make torture and each needs specific --unmask flags added to internal/invariants/torture_catalogue_test.go's tortureSchemas: gitlab (at least six objects, docs/TORTURE.md names the first six -- a recurring Rails polymorphic-_type-discriminator-in-a-partial-index/CHECK shape that likely recurs further, given its schema is the largest of the ten), odoo (public.ir_filters.ir_filters_name_model_uid_unique_action_index, a COALESCE(...,'-1'::integer) sentinel), discourse (public.categories.unique_index_categories_on_name, the same COALESCE('-1'::integer) shape), and supabase-auth (auth.custom_oauth_providers.custom_oauth_providers_oauth2_requires_endpoints, where clearing it trades off --unmask on the constraint's other masked columns -- authorization_url/userinfo_url/token_url -- and needs a real judgement call about that schema, not a mechanical flag add). Update internal/invariants/torture_test.go's --unmask count and docs/TORTURE.md's own section once each schema is green again.

## Acceptance



## Log

- 2026-09-16 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
