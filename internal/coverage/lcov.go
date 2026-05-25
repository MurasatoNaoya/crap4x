// Package coverage parses lcov coverage reports.
package coverage

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

// Parse reads lcov data from r and returns a map of file path to
// (line number -> hit count) for every instrumented line.
// Unknown record types are silently ignored.
// Malformed DA lines are skipped without returning an error.
// A missing final end_of_record is tolerated; any in-progress file is included.
func Parse(r io.Reader) (map[string]map[int]int, error) {
	result := make(map[string]map[int]int)

	var currentFile string
	var currentLines map[int]int

	finalise := func() {
		if currentFile != "" && currentLines != nil {
			result[currentFile] = currentLines
		}
		currentFile = ""
		currentLines = nil
	}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "SF:"):
			finalise()
			currentFile = strings.TrimPrefix(line, "SF:")
			currentLines = make(map[int]int)

		case strings.HasPrefix(line, "DA:"):
			if currentLines == nil {
				continue
			}
			parts := strings.SplitN(strings.TrimPrefix(line, "DA:"), ",", 2)
			if len(parts) != 2 {
				continue // malformed — skip
			}
			lineNo, err := strconv.Atoi(parts[0])
			if err != nil {
				continue // malformed line number — skip
			}
			hits, err := strconv.Atoi(parts[1])
			if err != nil {
				continue // malformed hit count — skip
			}
			currentLines[lineNo] = hits

		case line == "end_of_record":
			finalise()
		}
		// All other record types (TN, FN, FNDA, FNF, FNH, BRDA, BRF, BRH, LF, LH) are ignored.
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Flush any file that didn't have a trailing end_of_record.
	finalise()

	return result, nil
}
