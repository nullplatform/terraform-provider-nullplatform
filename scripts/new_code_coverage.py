#!/usr/bin/env python3
"""The coverage gate: two rules, one run.

1. NEW code is fully covered — the lines `git diff <base>...HEAD` added,
   intersected with the never-executed statements in the coverprofile,
   must be empty (minus the documented-unreachable entries in
   scripts/coverage_accepted.txt).
2. TOTAL coverage never falls — the profile's statement coverage must be
   at or above the floor in scripts/coverage_floor.txt. The floor is a
   RATCHET, not a target: it only ever rises. When a branch lifts the
   total meaningfully above it, raise the floor in the same PR. It is
   deliberately not "every commit must improve the total": deleting
   well-covered dead code and pure refactors are legitimate and flat.

Together they make the total climb monotonically — every change is born
covered, so new code can only dilute the uncovered remainder — without
punishing deletions or refactors.

Usage:
    go test ./... -coverprofile=coverage.out
    python3 scripts/new_code_coverage.py [base-ref] [profile]

Defaults: base-ref origin/main, profile coverage.out.
Test files never count; only *.go under version control do.
"""

import re
import subprocess
import sys
from collections import defaultdict

BASE = sys.argv[1] if len(sys.argv) > 1 else "origin/main"
PROFILE = sys.argv[2] if len(sys.argv) > 2 else "coverage.out"
ACCEPTED = "scripts/coverage_accepted.txt"
FLOOR = "scripts/coverage_floor.txt"


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


def total_statement_coverage(module):
    """Covered / total statements across the module, as a percentage."""
    covered = total = 0
    prefix = module + "/"
    with open(PROFILE) as fh:
        for line in fh:
            line = line.strip()
            if not line or line.startswith("mode:"):
                continue
            location, _, count = line.rpartition(" ")
            location, _, statements = location.rpartition(" ")
            if not location.startswith(prefix):
                continue
            total += int(statements)
            if int(count) != 0:
                covered += int(statements)
    return 100.0 * covered / total if total else 100.0


def check_floor(module):
    """Rule 2: the ratchet. Returns an error message or None."""
    try:
        with open(FLOOR) as fh:
            floor = float(fh.read().split("#", 1)[0].strip())
    except FileNotFoundError:
        return None
    coverage = total_statement_coverage(module)
    print(f"total statement coverage: {coverage:.1f}%   floor: {floor:.1f}%")
    if coverage < floor:
        return (
            f"total coverage {coverage:.1f}% fell below the floor {floor:.1f}% "
            f"({FLOOR}). Cover what this change uncovered, or — only if the drop "
            f"comes from DELETING covered code — lower the floor in the same PR "
            f"and say so in the commit message."
        )
    if coverage >= floor + 1.0:
        print(
            f"note: coverage is {coverage - floor:.1f} points above the floor — "
            f"consider raising {FLOOR} to {coverage:.1f} in this PR"
        )
    return None


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
    module = module_path()
    added = added_lines_by_file()
    uncovered = uncovered_lines_by_file(module)
    accepted = accepted_lines_by_file()
    floor_error = check_floor(module)
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
    for path, missing in gaps:
        print(f"{path}")
        print(f"   uncovered: {compact(missing)}\n")
    if not gaps:
        print("every line this branch added is covered")
    if floor_error:
        print(f"\n{floor_error}")
    if gaps or floor_error:
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
