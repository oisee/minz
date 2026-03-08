package plm

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PreprocessFile is like Preprocess but resolves $INCLUDE directives by
// searching for the included file in baseDir (the directory of the main file).
// CP/M device designators (:Fn:) in the include path are stripped.
func PreprocessFile(src, baseDir string) string {
	src = resolveIncludes(src, baseDir)
	return Preprocess(src)
}

// resolveIncludes replaces $INCLUDE(:device:file) lines with the contents of
// the included file, searching in baseDir.  Unresolvable includes are blanked.
func resolveIncludes(src, baseDir string) string {
	// $INCLUDE(:F1:FILENAME.LIT) or $INCLUDE(FILENAME.LIT)
	includeRE := regexp.MustCompile(`(?i)\$INCLUDE\s*\(([^)]*)\)`)
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		m := includeRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		raw := m[1]
		// Strip CP/M device designator :Xn: prefix.
		if idx := strings.LastIndex(raw, ":"); idx >= 0 {
			raw = raw[idx+1:]
		}
		raw = strings.TrimSpace(raw)
		// Try case-insensitive file lookup in baseDir.
		included := findFileCI(baseDir, raw)
		if included == "" {
			lines[i] = strings.Repeat(" ", len(line))
			continue
		}
		data, err := os.ReadFile(included)
		if err != nil {
			lines[i] = strings.Repeat(" ", len(line))
			continue
		}
		lines[i] = string(data)
	}
	return strings.Join(lines, "\n")
}

// findFileCI looks for a file named name (case-insensitively) in dir.
func findFileCI(dir, name string) string {
	lower := strings.ToLower(name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if strings.ToLower(e.Name()) == lower {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

// Preprocess performs PL/M-80 source preprocessing before lexing:
//
//  1. Strips $compiler-control lines ($SET, $INCLUDE, $U, etc.)
//  2. Strips block comments /* ... */
//  3. Collects all DECLARE … LITERALLY '…'; directives (single and multi-name,
//     including cases where the LITERALLY keyword was itself defined via a
//     previous LITERALLY — handled by within-block fixpoint expansion and
//     iterating until stable).
//  4. Removes those declarations from the source text.
//  5. Replaces every occurrence of the defined names (as whole words) with
//     the substitution text.
func Preprocess(src string) string {
	upper := strings.ToUpper(src)
	upper = stripDollarDirectives(upper)

	// Iteratively resolve LITERALLY aliases.
	//
	// Each pass:
	//   1. Collect NAME→text pairs from all DECLARE...LITERALLY blocks.
	//      Within-block fixpoint expansion handles same-block alias chains
	//      (e.g. LIT LITERALLY 'LITERALLY' and FOREVER LIT 'WHILE TRUE' in
	//      one DECLARE block: after seeing LIT→LITERALLY we re-expand the
	//      block to find FOREVER→WHILE TRUE).
	//   2. Blank those DECLARE blocks (preserving newlines for line numbers).
	//   3. Apply the collected substitutions to the full source.
	// Repeat until no new blocks are found.
	for pass := 0; pass < 8; pass++ {
		subs, blanks := collectLiterally(upper)
		if len(blanks) == 0 {
			break
		}
		buf := []byte(upper)
		for _, b := range blanks {
			for j := b[0]; j < b[1]; j++ {
				if buf[j] != '\n' {
					buf[j] = ' '
				}
			}
		}
		upper = string(buf)
		for _, s := range subs {
			upper = replaceWord(upper, s[0], s[1])
		}
	}
	return upper
}

// collectLiterally finds all NAME LITERALLY 'text' pairs in DECLARE blocks.
// Returns pairs [][2]string and the byte spans to blank out.
//
// Within-block fixpoint expansion: after extracting any pair from a block, the
// accumulated subs are applied to that block's text and the regex is re-run.
// This handles same-block alias chains such as:
//
//	DECLARE LIT     LITERALLY 'LITERALLY',
//	        FOREVER LIT       'WHILE TRUE';
//
// On the first pass LIT→LITERALLY is found; on the second pass the block text
// is expanded so FOREVER LITERALLY 'WHILE TRUE' becomes visible.
func collectLiterally(s string) ([][2]string, [][2]int) {
	onePairRE := regexp.MustCompile(
		`(?i)\b([A-Z_$][A-Z0-9_$]*)\s+LITERALLY\s+'([^']*)'`)

	// Match any DECLARE....; block that contains at least one LITERALLY pair.
	// Uses a lazy match: [^;']* for non-quote/non-semicolon content, and
	// '[^']*' for quoted strings, until we see LITERALLY followed by a quote.
	// The (?s) flag lets . match newlines.
	declLitRE := regexp.MustCompile(
		`(?is)DECLARE\s+(?:[^;']|'[^']*')*?\bLITERALLY\s+'[^']*'` + // up to first LITERALLY
			`(?:[^;']|'[^']*')*;`) // rest until semicolon

	var subs [][2]string
	var blanks [][2]int
	seen := make(map[string]bool)

	matches := declLitRE.FindAllStringIndex(s, -1)
	for _, m := range matches {
		blockText := s[m[0]:m[1]]

		// Within-block fixpoint: apply currently-known subs to the block text,
		// extract new pairs, repeat until no new pairs are found.
		for {
			expanded := blockText
			for _, sub := range subs {
				expanded = replaceWord(expanded, sub[0], sub[1])
			}
			pairs := onePairRE.FindAllStringSubmatch(expanded, -1)
			addedHere := false
			for _, pair := range pairs {
				name := strings.ToUpper(pair[1])
				if !seen[name] {
					seen[name] = true
					subs = append(subs, [2]string{name, pair[2]})
					addedHere = true
				}
			}
			if !addedHere {
				break // no new subs from this block — done
			}
			// New subs added; re-expand the block text and look for more.
		}
		blanks = append(blanks, [2]int{m[0], m[1]})
	}
	return subs, blanks
}

// stripDollarDirectives blanks out PL/M compiler control lines.
// A control line is any line whose first non-space content starts with '$'.
func stripDollarDirectives(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t\r")
		if strings.HasPrefix(trimmed, "$") {
			lines[i] = strings.Repeat(" ", len(line))
		}
	}
	return strings.Join(lines, "\n")
}

// stripBlockComments removes /* ... */ comments, preserving newlines for
// accurate line-number reporting.  Single-quoted PL/M string literals
// ('...') are passed through unchanged so that '/*' inside a string is
// not misinterpreted as a comment start.
func stripBlockComments(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		// Skip single-quoted string literals intact.
		if s[i] == '\'' {
			b.WriteByte(s[i])
			i++
			for i < len(s) && s[i] != '\'' {
				b.WriteByte(s[i])
				i++
			}
			if i < len(s) {
				b.WriteByte(s[i]) // closing '
				i++
			}
			continue
		}
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			i += 2
			for i < len(s) {
				if i+1 < len(s) && s[i] == '*' && s[i+1] == '/' {
					i += 2
					break
				}
				if s[i] == '\n' {
					b.WriteByte('\n')
				} else {
					b.WriteByte(' ')
				}
				i++
			}
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// replaceWord replaces all whole-word occurrences of old with newText in s.
func replaceWord(s, old, newText string) string {
	if old == "" {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		idx := strings.Index(s[i:], old)
		if idx < 0 {
			b.WriteString(s[i:])
			break
		}
		abs := i + idx
		// Check left boundary.
		if abs > 0 && isIdentChar(s[abs-1]) {
			b.WriteByte(s[i])
			i++
			continue
		}
		// Check right boundary.
		end := abs + len(old)
		if end < len(s) && isIdentChar(s[end]) {
			b.WriteString(s[i : abs+1])
			i = abs + 1
			continue
		}
		b.WriteString(s[i:abs])
		b.WriteString(newText)
		i = end
	}
	return b.String()
}

func isIdentChar(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') || c == '_' || c == '$'
}
