# GitHub Copilot Project Instructions

You are assisting in this repository. The following rules are **mandatory** and override any defaults.

---

## General Behavior

1. **Read full files, not fragments**

   - When you open a file, always consider the **entire file** contents, not just the visible or edited region.
   - Before refactoring or implementing features, scan all relevant files fully (including related modules) to understand existing patterns, types, and architecture.
   - Do not assume behavior from partial context if you can open and read the whole file instead.
   - Read files from other gGitHub repositories as needed.
   - Read issues/pull requests from remote repositories as needed.

2. **No artificial time/phasing limits**

   - Do **not** shorten work or split it into artificial “phases” just to finish quickly.
   - Always attempt to provide the **most complete, fully implemented solution** you reasonably can in a single pass:
     - Implement full feature logic instead of stubs.
     - Add tests when appropriate.
     - Update related documentation/configs only if clearly required.
   - Avoid responses like “I’ll do the rest later” or “this is just a partial implementation” unless explicitly requested.

3. **Never leave partially implemented code**

   - Do **not** leave functions, classes, or modules partially implemented.
   - Always provide a complete implementation for any code you modify or add.
   - If you cannot fully implement something due to missing context, dependencies, or constraints, clearly state what is missing and provide a best-effort implementation based on available information.
   - In such cases ask for missing context and wait for my response before proceeding further.

---

## Testing & Validation

5. **Always test changes**

   - Whenever you modify code, **run tests** relevant to the change to ensure correctness.
   - If you add or update tests:
     - Execute the new or modified tests to confirm they pass.
     - Ensure coverage reflects the newly added logic.
   - When presenting results:
     - Output plain test results or shell-style logs.
     - **Do not generate fancy Markdown summaries**, reports, tables, or “change overview” documents. outputting this in console is OK.
     - A simple textual summary or shell-like output is sufficient (e.g., which tests passed/failed).

---

## Git / Commits

3. **Do NOT generate commits**

   - Never run Git commands that create commits, amend commits, or push changes.
   - Do **not** auto-generate commit messages or suggest commands like:
     - `git commit ...`
     - `git commit -m "..."`
     - `git commit --amend`
     - `git push`
   - I will create commits myself; they must be signed (`-S`) and signed-off (`-s`), so leave all commit creation to me.

   - It is acceptable to:
     - Modify files in the working tree.
     - Describe *what* should be committed, but not *how* to commit.

---

## File / Directory Access

4. **Repository file access and permissions**

   - You may read any file and directory within this repository and other repositories as needed.
   - **Never ask for additional read permissions** for items inside this repo; assume full access.
   - Locate relevant code by searching the repo, exploring related modules, tests, and configs. Fetch data from remote if needed.
   - If something is outside this repository entirely, its also OK to read it.

---

## Style of Assistance

- Follow the repository’s existing style, patterns, conventions, and architecture.
- Avoid unnecessary churn or unrelated refactoring.
- Explain non-obvious design decisions briefly in comments when helpful.
- When uncertain, produce a **best-effort, concrete implementation** using the available code rather than asking unnecessary questions.

