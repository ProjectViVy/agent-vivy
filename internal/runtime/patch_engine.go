package runtime

import (
	"fmt"
	"strings"
)

// applyStringPatch locates oldString in content and returns the patched
// content plus the number of replaced occurrences.
//
// Exact substring matching comes first. When it finds nothing, a bounded
// whitespace-tolerant fallback runs: the file region must match oldString
// line-for-line with leading and trailing whitespace ignored (a trailing
// newline on either side does not anchor the match to a following blank
// line), and the replacement keeps the file's own indentation on every line
// whose new_string indentation equals the old_string indentation (a model
// that guessed the indentation gets the file's; a model that deliberately
// reindented keeps its own). Replacements whose line count differs from the
// matched region are inserted verbatim. Line endings follow the file: a
// matched line's trailing CR is transferred to the corresponding inserted
// line so CRLF files stay consistent.
func applyStringPatch(content, oldString, newString string, replaceAll bool) (string, int, error) {
	if oldString == "" {
		return "", 0, fmt.Errorf("filesystem: old_string must not be empty")
	}
	if count := strings.Count(content, oldString); count > 0 {
		if !replaceAll && count != 1 {
			return "", 0, fmt.Errorf("filesystem: old_string matched %d times; set replace_all=true to replace all", count)
		}
		n := 1
		if replaceAll {
			n = -1
		}
		return strings.Replace(content, oldString, newString, n), count, nil
	}
	return applyWhitespaceTolerantPatch(content, oldString, newString, replaceAll)
}

// applyWhitespaceTolerantPatch is the fallback matcher: line-anchored,
// whitespace-insensitive per line, with the indentation-transfer rules from
// applyStringPatch. Ambiguity is reported the same way as the exact path.
func applyWhitespaceTolerantPatch(content, oldString, newString string, replaceAll bool) (string, int, error) {
	contentLines := strings.Split(content, "\n")
	oldLines := strings.Split(oldString, "\n")
	newLines := strings.Split(newString, "\n")
	// A trailing newline must not anchor the match to a following blank
	// line: drop the empty final element it creates on each side, so
	// "a\nb\n" matches the first two lines whose contents trim to a and b.
	if strings.HasSuffix(oldString, "\n") && len(oldLines) > 0 && oldLines[len(oldLines)-1] == "" {
		oldLines = oldLines[:len(oldLines)-1]
	}
	if strings.HasSuffix(newString, "\n") && len(newLines) > 0 && newLines[len(newLines)-1] == "" {
		newLines = newLines[:len(newLines)-1]
	}
	if len(oldLines) > len(contentLines) {
		return "", 0, fmt.Errorf("filesystem: old_string was not found")
	}
	var starts []int
	for i := 0; i+len(oldLines) <= len(contentLines); i++ {
		if regionMatches(contentLines[i:i+len(oldLines)], oldLines) {
			// Left-to-right, non-overlapping, like strings.Replace.
			if len(starts) == 0 || i >= starts[len(starts)-1]+len(oldLines) {
				starts = append(starts, i)
			}
		}
	}
	if len(starts) == 0 {
		return "", 0, fmt.Errorf("filesystem: old_string was not found")
	}
	if !replaceAll && len(starts) != 1 {
		return "", 0, fmt.Errorf("filesystem: old_string matched %d times (whitespace-insensitive); set replace_all=true to replace all", len(starts))
	}
	for k := len(starts) - 1; k >= 0; k-- {
		start := starts[k]
		end := start + len(oldLines)
		repl := replacementLines(contentLines[start:end], oldLines, newLines)
		contentLines = append(contentLines[:start], append(repl, contentLines[end:]...)...)
	}
	return strings.Join(contentLines, "\n"), len(starts), nil
}

func regionMatches(region, oldLines []string) bool {
	for j, ol := range oldLines {
		if strings.TrimSpace(region[j]) != strings.TrimSpace(ol) {
			return false
		}
	}
	return true
}

// replacementLines builds the lines that replace one matched region.
func replacementLines(region, oldLines, newLines []string) []string {
	out := append([]string(nil), newLines...)
	for j := range out {
		if j >= len(region) {
			break // lines beyond the matched region: keep verbatim
		}
		// Keep the file's line ending: a CRLF line stays CRLF.
		if strings.HasSuffix(region[j], "\r") && !strings.HasSuffix(out[j], "\r") {
			out[j] += "\r"
		}
		if len(newLines) != len(oldLines) {
			continue // no line correspondence: insert verbatim
		}
		fileLeading := leadingWhitespace(region[j])
		modelOldLeading := leadingWhitespace(oldLines[j])
		switch {
		case fileLeading == modelOldLeading:
			// The model reproduced the file's indentation; keep its line.
		case leadingWhitespace(newLines[j]) == modelOldLeading:
			// The model kept its (wrong) indentation: restore the file's.
			out[j] = fileLeading + strings.TrimLeft(out[j], " \t")
		default:
			// The model reindented deliberately; keep its line.
		}
	}
	return out
}

func leadingWhitespace(line string) string {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return line[:i]
}
