# EasyCLA: Author and Co-author Caching + Large-PR Support

- **Two-level caching** for author and co-author identity & identity plus per-project signature decisions.
- **GraphQL-based commit ingestion** that comfortably handles PRs with **250+ commits (and beyond)**.

---

## Why it matters
- Faster PR checks and `/easycla` re-runs.
- Lower DB/API load via memoized decisions.
- Stable, deterministic output and accurate status posting on the PR **head SHA**.

---

## Caching 
- **General cache key**: `(author_id, lower(login), lower(email)) → (user | None)`
- **Per-project cache key**: `(project_id, author_id, lower(login), lower(email)) → (user | None, authorized, affiliated)`
- **TTL policy**: positives **~24h**; negative/uncertain states use **Quick TTL = 5m**.
- **Flow**: per-project cache → general cache → cold DB path. Results are stored back with the appropriate TTL.
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

## Quick constants
- `QUICK_CACHE_TTL = 300` seconds (negative/uncertain states).
- Default positive cache TTL ≈ **24 hours**.
- GraphQL: `pageSize=100`, parallel workers tuned for throughput.
