# EasyCLA: Author and Co-author Caching + Large-PR Support

- **Two-level caching** for author and co-author identity & identity plus per-project signature decisions.
- **Caching** for git co-authors parsed from commit messages.
- **GraphQL-based commit ingestion** that comfortably handles PRs with **250+ commits (and beyond)**.

---

## Why it matters
- Faster PR checks and `/easycla` re-runs.
- Lower DB/API load via memoized decisions.
- Stable, deterministic output and accurate status posting on the PR **head SHA**.

---

## Caching 
- **Co-authors cache key** are cached based on their commit traile's (`Co-authored-by:`) normalized email and name.
- **General cache key**: `(author_id, lower(login), lower(email)) → (user | None)`
- **Per-project cache key**: `(project_id, author_id, lower(login), lower(email)) → (user | None, authorized, affiliated)`
- **TTL policy**: positives **~12h** (**~3h** for per-project with signature status); negative/uncertain states use **Negative TTL = 3m**.
- **Flow**: per-project cache → general cache → cold DB path. Results are stored back with the appropriate TTL.
- **When signature is signed**: per-project and general caches are updated to reflect the new status (general cache is updated because given user could have no DynamoDB entry yet before signing the CLA).
- There are /v2/clear-cache and /v4/clear-cache endpoints to clear the caches (for testing and operational purposes).
- Thread-safe with periodic expired entries cleanup (once per hour).

---

## Large PR (250+) support
- Switch to **GitHub GraphQL** for commits (`pageSize=100`) with cursor paging.
- Parallel processing via thread pool; co-authors parsed from **commit messages** (`Co-authored-by:`).
- Final actor lists are **de-duplicated** and **sorted** (login, name, email, sha) for stable comments.
- PR comments are **edited only when normalized body changes** (prevents churn & size bloat).
- Commit statuses are always posted to the **true PR head SHA**.

---

## Operational notes
- Expect noticeable **latency reduction** on large PRs and repeated checks.
- Fallbacks remain safe; unknown users land in an “Unknown” bucket with guidance.
- No behavior change to the core signing rules—only faster execution.

---

## Constants
- `NEGATIVE_CACHE_TTL = 180` seconds (negative/uncertain states).
- Default positive cache TTL ≈ **12 hours** (**3 hours** for per-project with signature status).
- GraphQL: `pageSize=100`, parallel workers tuned for throughput.
