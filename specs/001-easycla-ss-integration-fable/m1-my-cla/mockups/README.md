<!--
Copyright The Linux Foundation and each contributor to CommunityBridge.
SPDX-License-Identifier: CC-BY-4.0
-->

# M1 "My CLAs" — UI mockups

Static HTML design references for the read-only "My CLAs" view (Me lens → Profile tab).
Not production code — reference only for reviewers and the Angular implementation (T020–T022).

- `my-clas-populated.html` — populated table: ICLA rows with **Download PDF**, ECLA rows
  showing **Covered by Corporate CLA (CCLA)** (no PDF), project/type/signed-date columns.
- `my-clas-empty-state.html` — empty state ("No CLAs on file yet") with identity-linking prompt.

Both confirm M1 spec decisions: My CLAs as a Profile tab, ICLA vs ECLA distinction,
PDF download for ICLA only, and the "link your Email/GitHub/GitLab" identity prompt.
