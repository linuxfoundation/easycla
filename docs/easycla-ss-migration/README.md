<!-- Copyright The Linux Foundation and each contributor to CommunityBridge.
SPDX-License-Identifier: CC-BY-4.0 -->

# EasyCLA → LFX Self Serve Migration — Architecture Review

Self-contained materials for the architecture review of the EasyCLA-to-Self-Serve migration. Reading order:

1. **[architecture-proposal.md](architecture-proposal.md)** — start here. Current state, milestones, what leadership already settled, the proposed architecture (P1–P8), top risks, and what the review should challenge.
2. **[role-mapping-feasibility.md](role-mapping-feasibility.md)** — the supporting deep analysis for the roles/permissions bridge (P2/P3): how EasyCLA v4 authorization actually works, token paths, read paths, options assessment, and the spike list. All claims cite `file:line`.
3. **[slides/easycla-ss-integration.pptx](slides/easycla-ss-integration.pptx)** — slide deck.

Implementation-level specifications (milestone scopes, acceptance criteria, per-milestone plans — used by the Spec Kit workflow) live separately in [specs/001-easycla-ss-integration-fable/](../../specs/001-easycla-ss-integration-fable/spec.md). This folder is for evaluating the architecture; that folder is for building it.
