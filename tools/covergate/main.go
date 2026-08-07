// Command covergate is the repository's coverage gate: two rules, one run.
//
//  1. NEW code is fully covered — the lines `git diff <base>...HEAD` added
//     under nullplatform/, intersected with the never-executed statements in
//     the coverprofile, must be empty (minus the documented-unreachable
//     entries in scripts/coverage_accepted.txt).
//  2. TOTAL coverage never falls — the profile's statement coverage must be
//     at or above the floor in scripts/coverage_floor.txt. The floor is a
//     RATCHET, not a target: it only ever rises. When a branch lifts the
//     total meaningfully above it, raise the floor in the same PR. It is
//     deliberately not "every commit must improve the total": deleting
//     well-covered dead code and pure refactors are legitimate and flat.
//
// Together they make the total climb monotonically — every change is born
// covered, so new code can only dilute the uncovered remainder — without
// punishing deletions or refactors.
//
// Usage:
//
//	go test ./nullplatform/ -coverprofile=coverage.out
//	go run ./tools/covergate [-base origin/main] [-profile coverage.out]
//
// Only *.go files under nullplatform/ count (that is what the profile
// measures); _test.go files never do.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	acceptedFile = "scripts/coverage_accepted.txt"
	floorFile    = "scripts/coverage_floor.txt"
	measuredDir  = "nullplatform/"
)

type lineSet map[int]struct{}

func (s lineSet) add(from, to int) {
	for i := from; i <= to; i++ {
		s[i] = struct{}{}
	}
}

func main() {
	base := flag.String("base", "origin/main", "ref the branch is diffed against")
	profile := flag.String("profile", "coverage.out", "go test -coverprofile output")
	flag.Parse()

	module, err := modulePath()
	must(err)
	added, err := addedLines(*base)
	must(err)
	uncovered, covered, total, err := profileLines(*profile, module)
	must(err)
	accepted, err := acceptedLines()
	must(err)

	// Rule 2 first, so its verdict prints even when rule 1 fails too.
	totalPct := 100.0
	if total > 0 {
		totalPct = 100 * float64(covered) / float64(total)
	}
	floorErr := checkFloor(totalPct)

	acceptedHits := 0
	for path, lines := range accepted {
		for line := range lines {
			if _, ok := uncovered[path][line]; ok {
				delete(uncovered[path], line)
				acceptedHits++
			}
		}
	}

	totalAdded, gapCount := 0, 0
	gaps := map[string][]int{}
	for path, lines := range added {
		totalAdded += len(lines)
		for line := range lines {
			if _, ok := uncovered[path][line]; ok {
				gaps[path] = append(gaps[path], line)
				gapCount++
			}
		}
	}

	fmt.Printf("base: %s\n", *base)
	fmt.Printf("total statement coverage: %.1f%%\n", totalPct)
	fmt.Printf("added lines: %d   uncovered among them: %d   (+%d accepted as unreachable, see %s)\n\n",
		totalAdded, gapCount, acceptedHits, acceptedFile)

	for _, path := range sortedKeys(gaps) {
		sort.Ints(gaps[path])
		fmt.Printf("%s\n   uncovered: %s\n\n", path, compact(gaps[path]))
	}
	if gapCount == 0 {
		fmt.Println("every line this branch added is covered")
	}
	if floorErr != nil {
		fmt.Printf("\n%v\n", floorErr)
	}
	if gapCount > 0 || floorErr != nil {
		os.Exit(1)
	}
}

func modulePath() (string, error) {
	fh, err := os.Open("go.mod")
	if err != nil {
		return "", err
	}
	defer fh.Close()
	scanner := bufio.NewScanner(fh)
	for scanner.Scan() {
		if module, found := strings.CutPrefix(scanner.Text(), "module "); found {
			return strings.TrimSpace(module), nil
		}
	}
	return "", fmt.Errorf("go.mod: no module line")
}

var hunkHeader = regexp.MustCompile(`^@@ .*\+(\d+)(?:,(\d+))? @@`)

// addedLines maps repo-relative measured *.go paths to the line numbers the
// branch added, read from a zero-context diff against the merge base.
func addedLines(base string) (map[string]lineSet, error) {
	out, err := exec.Command("git", "diff", "-U0", base+"...HEAD", "--",
		measuredDir+"*.go", ":!"+measuredDir+"*_test.go").Output()
	if err != nil {
		return nil, fmt.Errorf("git diff against %s: %w", base, err)
	}
	added := map[string]lineSet{}
	var path string
	for _, line := range strings.Split(string(out), "\n") {
		if file, found := strings.CutPrefix(line, "+++ b/"); found {
			path = file
			continue
		}
		match := hunkHeader.FindStringSubmatch(line)
		if match == nil || path == "" {
			continue
		}
		start, _ := strconv.Atoi(match[1])
		count := 1
		if match[2] != "" {
			count, _ = strconv.Atoi(match[2])
		}
		if count == 0 {
			continue
		}
		if added[path] == nil {
			added[path] = lineSet{}
		}
		added[path].add(start, start+count-1)
	}
	return added, nil
}

// profileLines reads a coverprofile into (uncovered lines per file, covered
// statements, total statements). A line shared by a covered and an uncovered
// block — an `if err != nil {` header — counts as covered, matching what
// `go tool cover -html` paints.
func profileLines(profile, module string) (map[string]lineSet, int, int, error) {
	fh, err := os.Open(profile)
	if err != nil {
		return nil, 0, 0, err
	}
	defer fh.Close()

	prefix := module + "/"
	uncovered := map[string]lineSet{}
	coveredLines := map[string]lineSet{}
	coveredStatements, totalStatements := 0, 0

	scanner := bufio.NewScanner(fh)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		// <file>:<startLine>.<startCol>,<endLine>.<endCol> <numStmt> <count>
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, 0, 0, fmt.Errorf("%s: unparseable line %q", profile, line)
		}
		file, span, found := strings.Cut(fields[0], ":")
		if !found || !strings.HasPrefix(file, prefix) {
			continue
		}
		statements, _ := strconv.Atoi(fields[1])
		count, _ := strconv.Atoi(fields[2])
		totalStatements += statements
		if count != 0 {
			coveredStatements += statements
		}

		startPart, endPart, _ := strings.Cut(span, ",")
		start, _ := strconv.Atoi(strings.SplitN(startPart, ".", 2)[0])
		end, _ := strconv.Atoi(strings.SplitN(endPart, ".", 2)[0])
		target := uncovered
		if count != 0 {
			target = coveredLines
		}
		path := strings.TrimPrefix(file, prefix)
		if target[path] == nil {
			target[path] = lineSet{}
		}
		target[path].add(start, end)
	}
	for path, lines := range coveredLines {
		for line := range lines {
			delete(uncovered[path], line)
		}
	}
	return uncovered, coveredStatements, totalStatements, scanner.Err()
}

var acceptedEntry = regexp.MustCompile(`^([^:\s]+):(\d+)(?:-(\d+))?$`)

func acceptedLines() (map[string]lineSet, error) {
	fh, err := os.Open(acceptedFile)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]lineSet{}, nil
		}
		return nil, err
	}
	defer fh.Close()

	accepted := map[string]lineSet{}
	scanner := bufio.NewScanner(fh)
	for scanner.Scan() {
		entry, _, _ := strings.Cut(scanner.Text(), "#")
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		match := acceptedEntry.FindStringSubmatch(entry)
		if match == nil {
			return nil, fmt.Errorf("%s: unparseable entry %q", acceptedFile, entry)
		}
		start, _ := strconv.Atoi(match[2])
		end := start
		if match[3] != "" {
			end, _ = strconv.Atoi(match[3])
		}
		if accepted[match[1]] == nil {
			accepted[match[1]] = lineSet{}
		}
		accepted[match[1]].add(start, end)
	}
	return accepted, scanner.Err()
}

func checkFloor(totalPct float64) error {
	raw, err := os.ReadFile(floorFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	first, _, _ := strings.Cut(string(raw), "#")
	// The number sits on the first line; comment lines follow.
	floor, err := strconv.ParseFloat(strings.TrimSpace(strings.SplitN(first, "\n", 2)[0]), 64)
	if err != nil {
		return fmt.Errorf("%s: %w", floorFile, err)
	}
	fmt.Printf("coverage floor: %.1f%%\n", floor)
	if totalPct < floor {
		return fmt.Errorf("total coverage %.1f%% fell below the floor %.1f%% (%s). "+
			"Cover what this change uncovered, or — only if the drop comes from DELETING "+
			"covered code — lower the floor in the same PR and say so in the commit message",
			totalPct, floor, floorFile)
	}
	if totalPct >= floor+1.0 {
		fmt.Printf("note: coverage is %.1f points above the floor — consider raising %s to %.1f in this PR\n",
			totalPct-floor, floorFile, totalPct)
	}
	return nil
}

func sortedKeys(m map[string][]int) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// compact renders [1,2,3,7] as "1-3, 7".
func compact(lines []int) string {
	var parts []string
	start, previous := lines[0], lines[0]
	flush := func() {
		if start == previous {
			parts = append(parts, strconv.Itoa(start))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", start, previous))
		}
	}
	for _, line := range lines[1:] {
		if line != previous+1 {
			flush()
			start = line
		}
		previous = line
	}
	flush()
	return strings.Join(parts, ", ")
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
