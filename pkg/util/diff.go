package util

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
)

// GenerateDiff generates a unified diff between two files.
// srcPath is the staged (new) file, destPath is the existing (old) file.
// contextLines specifies how many lines of context to include around changes.
func GenerateDiff(srcPath, destPath string, contextLines int) (string, error) {
	srcContent, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("failed to read source file: %w", err)
	}

	var destContent []byte
	if IsFileExist(destPath) {
		destContent, err = os.ReadFile(destPath)
		if err != nil {
			return "", fmt.Errorf("failed to read destination file: %w", err)
		}
	}

	srcLines := splitLines(srcContent)
	destLines := splitLines(destContent)

	// Generate unified diff
	diff := unifiedDiff(destPath, srcPath, destLines, srcLines, contextLines)
	return diff, nil
}

// ColorizeDiff adds ANSI color codes to a diff string.
// Added lines (+) are green, removed lines (-) are red,
// and headers (@@) are cyan.
func ColorizeDiff(diff string) string {
	if diff == "" {
		return diff
	}

	var buf bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader(diff))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			// Header lines - bold
			buf.WriteString("\033[1m" + line + "\033[0m\n")
		case strings.HasPrefix(line, "@@"):
			// Hunk header - cyan
			buf.WriteString("\033[36m" + line + "\033[0m\n")
		case strings.HasPrefix(line, "+"):
			// Added lines - green
			buf.WriteString("\033[32m" + line + "\033[0m\n")
		case strings.HasPrefix(line, "-"):
			// Removed lines - red
			buf.WriteString("\033[31m" + line + "\033[0m\n")
		default:
			// Context lines - no color
			buf.WriteString(line + "\n")
		}
	}
	return buf.String()
}

// splitLines splits content into lines, handling various line endings.
func splitLines(content []byte) []string {
	if len(content) == 0 {
		return []string{}
	}
	s := string(content)
	// Normalize line endings
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	// Remove trailing empty line if file ends with newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// unifiedDiff generates a unified diff between two sets of lines.
// This is a simplified implementation that generates a reasonable diff
// for configuration files.
func unifiedDiff(oldName, newName string, oldLines, newLines []string, contextLines int) string {
	// Use longest common subsequence for diff
	lcs := computeLCS(oldLines, newLines)

	var hunks []hunk
	var currentHunk *hunk

	oldIdx, newIdx, lcsIdx := 0, 0, 0

	for oldIdx < len(oldLines) || newIdx < len(newLines) {
		// Check if current lines match LCS
		oldMatches := lcsIdx < len(lcs) && oldIdx < len(oldLines) && oldLines[oldIdx] == lcs[lcsIdx]
		newMatches := lcsIdx < len(lcs) && newIdx < len(newLines) && newLines[newIdx] == lcs[lcsIdx]

		if oldMatches && newMatches {
			// Lines match - context line
			if currentHunk != nil {
				currentHunk.lines = append(currentHunk.lines, diffLine{kind: ' ', content: oldLines[oldIdx]})
			}
			oldIdx++
			newIdx++
			lcsIdx++
		} else if !oldMatches && oldIdx < len(oldLines) {
			// Removed line
			if currentHunk == nil {
				currentHunk = &hunk{oldStart: oldIdx + 1, newStart: newIdx + 1}
			}
			currentHunk.lines = append(currentHunk.lines, diffLine{kind: '-', content: oldLines[oldIdx]})
			oldIdx++
		} else if !newMatches && newIdx < len(newLines) {
			// Added line
			if currentHunk == nil {
				currentHunk = &hunk{oldStart: oldIdx + 1, newStart: newIdx + 1}
			}
			currentHunk.lines = append(currentHunk.lines, diffLine{kind: '+', content: newLines[newIdx]})
			newIdx++
		}

		// Check if we should close the hunk (enough context after changes)
		if currentHunk != nil {
			contextCount := 0
			for i := len(currentHunk.lines) - 1; i >= 0; i-- {
				if currentHunk.lines[i].kind == ' ' {
					contextCount++
				} else {
					break
				}
			}
			if contextCount >= contextLines*2 && oldIdx < len(oldLines) {
				// Trim excess context
				currentHunk.lines = currentHunk.lines[:len(currentHunk.lines)-contextLines]
				hunks = append(hunks, *currentHunk)
				currentHunk = nil
			}
		}
	}

	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}

	if len(hunks) == 0 {
		return ""
	}

	// Format output
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("--- %s\n", oldName))
	buf.WriteString(fmt.Sprintf("+++ %s\n", newName))

	for _, h := range hunks {
		oldCount := 0
		newCount := 0
		for _, l := range h.lines {
			switch l.kind {
			case ' ':
				oldCount++
				newCount++
			case '-':
				oldCount++
			case '+':
				newCount++
			}
		}
		buf.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", h.oldStart, oldCount, h.newStart, newCount))
		for _, l := range h.lines {
			buf.WriteString(fmt.Sprintf("%c%s\n", l.kind, l.content))
		}
	}

	return buf.String()
}

type diffLine struct {
	kind    rune // ' ', '+', or '-'
	content string
}

type hunk struct {
	oldStart int
	newStart int
	lines    []diffLine
}

// computeLCS computes the longest common subsequence of two string slices.
func computeLCS(a, b []string) []string {
	m, n := len(a), len(b)
	if m == 0 || n == 0 {
		return nil
	}

	// Build LCS length table
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				if dp[i-1][j] > dp[i][j-1] {
					dp[i][j] = dp[i-1][j]
				} else {
					dp[i][j] = dp[i][j-1]
				}
			}
		}
	}

	// Backtrack to find LCS
	lcs := make([]string, 0, dp[m][n])
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			lcs = append(lcs, a[i-1])
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	// Reverse LCS
	for i, j := 0, len(lcs)-1; i < j; i, j = i+1, j-1 {
		lcs[i], lcs[j] = lcs[j], lcs[i]
	}

	return lcs
}
