#!/usr/bin/env python3
import argparse, os, sys, pathlib, io, subprocess, shutil

DEFAULT_EXCLUDE_DIRS = {".git", ".hg", ".svn", ".idea", ".vscode", "dist", "build", "out", "target", ".next", ".nuxt", ".tox", "__pycache__"}
DEFAULT_EXCLUDE_GLOBS = {"*.min.js", "*.map", "*.lock", "*.jar", "*.zip", "*.gz", "*.tgz", "*.bz2", "*.7z", "*.png",
                         "*.jpg", "*.jpeg", "*.gif", "*.webp", "*.ico", "*.pdf", "*.woff", "*.woff2", "*.ttf", "*.otf",
                         "*.mp4", "*.mov", "*.avi", "*.mp3", "*.flac", "*.wav", "*.iso", "*.bin", "*.secret"}
DEFAULT_INCLUDE_EXTS = {
    ".go",".ts",".tsx",".js",".jsx",".json",".yml",".yaml",".toml",".ini",".env",".md",".txt",
    ".proto",".graphql",".sql",".py",".rs",".java",".c",".h",".cpp",".hpp",".cc",".m",".mm",
    ".rb",".php",".pl",".sh",".bash",".zsh",".fish",".ps1",".bat",".dockerfile",".gradle",".properties"
}
ALSO_ALLOW_NAME = {"dockerfile", "makefile", "makefile.win"}

def is_text_file(path: str, max_probe=65536) -> bool:
    try:
        with open(path, "rb") as f:
            chunk = f.read(max_probe)
        if b"\x00" in chunk:
            return False
        chunk.decode("utf-8", errors="strict")
        return True
    except Exception:
        return False

def should_keep_by_ext(p: pathlib.Path, include_exts) -> bool:
    if include_exts and p.suffix.lower() not in include_exts:
        if p.name.lower() not in ALSO_ALLOW_NAME:
            return False
    return True

def should_exclude_by_glob(rel: str) -> bool:
    from pathlib import PurePath
    pp = PurePath(rel)
    for g in DEFAULT_EXCLUDE_GLOBS:
        if pp.match(g):
            return True
    return False

def git_available() -> bool:
    return shutil.which("git") is not None

def iter_files_git(repo_root: pathlib.Path):
    """
    Yields repo files not ignored by .gitignore/.git/info/exclude/global ignores.
    Uses: git ls-files --cached --others --exclude-standard
    """
    cmd = ["git", "-C", str(repo_root), "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--"]
    out = subprocess.check_output(cmd)
    for rel_b in out.split(b"\x00"):
        if not rel_b:
            continue
        rel = rel_b.decode("utf-8", errors="replace")
        p = repo_root / rel
        if p.is_file():
            yield p

def iter_files_walk(repo_root: pathlib.Path):
    """
    Fallback: walk the tree and filter manually (does NOT perfectly mirror .gitignore).
    """
    for root, dirs, files in os.walk(repo_root):
        # prune common junk dirs
        dirs[:] = [d for d in dirs if d not in DEFAULT_EXCLUDE_DIRS]
        for name in files:
            p = pathlib.Path(root) / name
            rel = p.relative_to(repo_root).as_posix()
            if should_exclude_by_glob(rel):
                continue
            yield p

def iter_repo_files(repo_root: pathlib.Path, use_git: bool):
    if use_git:
        yield from iter_files_git(repo_root)
    else:
        # use git if available and .git exists
        if (repo_root/".git").exists() and git_available():
            yield from iter_files_git(repo_root)
        else:
            yield from iter_files_walk(repo_root)

def write_repo(repo_root: pathlib.Path, out_prefix: pathlib.Path, max_mb: float, force_git: bool):
    repo_root = repo_root.resolve()
    max_bytes = int(max_mb * (1024**2))
    chunk_idx = 1
    bytes_in_chunk = 0

    def open_chunk(idx):
        suffix = "" if idx == 1 else f".part{idx}"
        path = out_prefix if idx == 1 else out_prefix.with_name(out_prefix.name + suffix)
        return path, io.open(path, "w", encoding="utf-8", newline="\n")

    out_path, fh = open_chunk(chunk_idx)

    count = 0
    for p in iter_repo_files(repo_root, force_git):
        rel = p.relative_to(repo_root).as_posix()
        if should_exclude_by_glob(rel):
            continue
        if not should_keep_by_ext(p, DEFAULT_INCLUDE_EXTS):
            continue
        if not is_text_file(str(p)):
            continue

        header = f"File: {rel}\nContents:\n"
        try:
            with io.open(p, "r", encoding="utf-8") as rf:
                content = rf.read()
        except UnicodeDecodeError:
            with io.open(p, "r", encoding="latin-1") as rf:
                content = rf.read()

        block = header + content.rstrip() + "\n\n"
        block_bytes = len(block.encode("utf-8"))

        if bytes_in_chunk + block_bytes > max_bytes and bytes_in_chunk > 0:
            fh.close()
            chunk_idx += 1
            out_path, fh = open_chunk(chunk_idx)
            bytes_in_chunk = 0

        fh.write(block)
        bytes_in_chunk += block_bytes
        count += 1

    fh.close()
    return chunk_idx, count

def main():
    ap = argparse.ArgumentParser(description="Dump repo sources to 'File: ...\\nContents:\\n...' format, honoring .gitignore.")
    ap.add_argument("--repo", default=".", help="Path to repo root (default: .)")
    ap.add_argument("--out", default="src.txt", help="Output filename/prefix")
    ap.add_argument("--max-mb", type=float, default=100.0, help="Max size per output file in MB (default: 100MB)")
    ap.add_argument("--git-mode", action="store_true", help="Force using 'git ls-files' (best accuracy for .gitignore).")
    args = ap.parse_args()

    repo_root = pathlib.Path(args.repo)
    out_prefix = pathlib.Path(args.out)

    try:
        chunks, files = write_repo(repo_root, out_prefix, args.max_mb, args.git_mode)
        print(f"Wrote {chunks} file(s); included {files} text source files. Upload the first file and any '.partN' files too.")
    except subprocess.CalledProcessError as e:
        print("Warning: Git mode failed; falling back to walk() (may include gitignored files).", file=sys.stderr)
        chunks, files = write_repo(repo_root, out_prefix, args.max_mb, force_git=False)
        print(f"Wrote {chunks} file(s); included {files} text source files. Upload the first file and any '.partN' files too.")

if __name__ == "__main__":
    sys.exit(main())

