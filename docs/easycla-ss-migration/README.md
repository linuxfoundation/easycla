<!-- Copyright The Linux Foundation and each contributor to CommunityBridge.
SPDX-License-Identifier: CC-BY-4.0 -->

# EasyCLA → LFX Self Serve Migration — Architecture Review

Self-contained materials for the architecture review of the EasyCLA-to-Self-Serve migration. Reading order:

1. **[architecture-proposal.md](architecture-proposal.md)** — the reviewed proposal (Eric Searcy, 2026-07-20). Current state, milestones, what leadership already settled, the proposed architecture (P1–P10, including the audit and trusted-caller designs), top risks, and what the review should challenge.
2. **[role-mapping-feasibility.md](role-mapping-feasibility.md)** — the supporting deep analysis for the roles/permissions bridge (P2/P3): how EasyCLA v4 authorization actually works, token paths, read paths, options assessment, and the spike list. All claims cite `file:line`.
3. **[m3-org-visibility.md](m3-org-visibility.md)** — why most EasyCLA companies are invisible in the Org Lens and what makes them visible (2026-09-10 call outcome, with prod data). **Reverses P2's "no CLA object types in OpenFGA before M5"** — read it against item 1.
4. **[Slide deck (Google Slides)](https://docs.google.com/presentation/d/1FQJOpiETIO_H10c6_eP2Zu-LM7qlvG_t7blhRm--2KA/edit)** — presentation for the review session.

Implementation-level specifications (milestone scopes, acceptance criteria, per-milestone plans — used by the Spec Kit workflow) live separately in [specs/001-easycla-ss-integration-fable/](../../specs/001-easycla-ss-integration-fable/spec.md). This folder is for evaluating the architecture; that folder is for building it.
