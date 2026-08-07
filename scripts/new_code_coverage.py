#!/usr/bin/env python3
"""Which lines THIS branch added are still uncovered?

The bar is coverage on the code a change introduces, not the repository's
historical total: take the added lines from `git diff <base>...HEAD`,
intersect them with the uncovered statements in a Go coverprofile, and
report only the intersection. Exits non-zero when uncovered added lines
remain, so CI can enforce it.

Usage:
    go test ./... -coverprofile=coverage.out
    python3 scripts/new_code_coverage.py [base-ref] [profile]

Defaults: base-ref origin/main, profile coverage.out.
Test files and generated docs never count; only *.go under version control do.
"""

import re
import subprocess
import sys
from collections import defaultdict

BASE = sys.argv[1] if len(sys.argv) > 1 else "origin/main"
PROFILE = sys.argv[2] if len(sys.argv) > 2 else "coverage.out"
ACCEPTED = "scripts/coverage_accepted.txt"


def module_path():
    with open("go.mod") as fh:
        for line in fh:
            if line.startswith("module "):
                return line.split()[1].strip()
    raise SystemExit("go.mod: no module line")


def added_lines_by_file():
    """Repo-relative *.go path -> set of line numbers this branch added."""
    diff = subprocess.run(
        ["git", "diff", "-U0", f"{BASE}...HEAD", "--", "*.go", ":!*_test.go"],
        capture_output=True,
        text=True,
        check=True,
    ).stdout
    added = defaultdict(set)
    path = None
    for line in diff.split("\n"):
        if line.startswith("+++ b/"):
            path = line[6:]
        elif line.startswith("@@") and path:
            match = re.search(r"\+(\d+)(?:,(\d+))?", line)
            if match:
                start = int(match.group(1))
                count = int(match.group(2) or 1)
                added[path].update(range(start, start + count))
    return added


def uncovered_lines_by_file(module):
    """Repo-relative path -> set of lines no executed statement touches.

    A line can belong to several blocks — `if err != nil {` sits in the
    covered condition AND the uncovered arm. Like `go tool cover -html`,
    a line counts as covered when ANY block touching it executed; only
    lines exclusively inside never-executed blocks are reported.
    """
    uncovered = defaultdict(set)
    covered = defaultdict(set)
    prefix = module + "/"
    with open(PROFILE) as fh:
        for line in fh:
            line = line.strip()
            if not line or line.startswith("mode:"):
                continue
            location, _, count = line.rpartition(" ")
            location, _, _ = location.rpartition(" ")
            file_part, _, span = location.partition(":")
            if not file_part.startswith(prefix):
                continue
            start, _, end = span.partition(",")
            lines = range(int(start.split(".")[0]), int(end.split(".")[0]) + 1)
            target = uncovered if int(count) == 0 else covered
            target[file_part[len(prefix):]].update(lines)
    for path, lines in covered.items():
        uncovered[path] -= lines
    return uncovered


def accepted_lines_by_file():
    """Documented-unreachable lines (scripts/coverage_accepted.txt)."""
    accepted = defaultdict(set)
    try:
        fh = open(ACCEPTED)
    except FileNotFoundError:
        return accepted
    with fh:
        for raw in fh:
            entry = raw.split("#", 1)[0].strip()
            if not entry:
                continue
            path, _, span = entry.partition(":")
            start, _, end = span.partition("-")
            accepted[path].update(range(int(start), int(end or start) + 1))
    return accepted


def main():
    added = added_lines_by_file()
    uncovered = uncovered_lines_by_file(module_path())
    accepted = accepted_lines_by_file()
    accepted_hits = 0
    for path, lines in accepted.items():
        accepted_hits += len(uncovered.get(path, set()) & lines)
        uncovered[path] -= lines

    total_added = sum(len(lines) for lines in added.values())
    gaps = []
    total_gap = 0
    for path in sorted(added):
        missing = sorted(added[path] & uncovered.get(path, set()))
        total_gap += len(missing)
        if missing:
            gaps.append((path, missing))

    print(f"base: {BASE}")
    print(
        f"added lines: {total_added}   uncovered among them: {total_gap}"
        f"   (+{accepted_hits} accepted as unreachable, see {ACCEPTED})\n"
    )
    if not gaps:
        print("every line this branch added is covered")
        return
    for path, missing in gaps:
        print(f"{path}")
        print(f"   uncovered: {compact(missing)}\n")
    sys.exit(1)


def compact(lines):
    """[1,2,3,7] -> '1-3, 7'"""
    ranges = []
    start = previous = lines[0]
    for line in lines[1:]:
        if line != previous + 1:
            ranges.append((start, previous))
            start = line
        previous = line
    ranges.append((start, previous))
    return ", ".join(str(a) if a == b else f"{a}-{b}" for a, b in ranges)


main()
