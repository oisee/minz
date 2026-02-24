package formats

import (
	"fmt"
	"strings"
)

// ZX Spectrum BASIC token table: keyword → token byte ($A5-$FF).
// Extracted from the 48K ROM at $0096.
// Longest keywords first for greedy matching.
var basicKeywords = []struct {
	keyword string
	token   byte
}{
	// Multi-word / longer keywords first (greedy match)
	{"RANDOMIZE", 0xF9},
	{"CONTINUE", 0xE8},
	{"RESTORE", 0xE5},
	{"INVERSE", 0xDD},
	{"SCREEN$", 0xAA},
	{"INKEY$", 0xA6},
	{"DEF FN", 0xCE},
	{"GO SUB", 0xED},
	{"GO TO", 0xEC},
	{"OPEN #", 0xD3},
	{"CLOSE #", 0xD4},
	{"BRIGHT", 0xDC},
	{"CIRCLE", 0xD8},
	{"BORDER", 0xE7},
	{"ERASE", 0xD2},
	{"FLASH", 0xDB},
	{"INPUT", 0xEE},
	{"MERGE", 0xD5},
	{"LPRINT", 0xE0},
	{"LLIST", 0xE1},
	{"PAPER", 0xDA},
	{"PAUSE", 0xF2},
	{"POINT", 0xA9},
	{"PRINT", 0xF5},
	{"CLEAR", 0xFD},
	{"VERIFY", 0xD6},
	{"FORMAT", 0xD0},
	{"RETURN", 0xFE},
	{"VAL$", 0xAE},
	{"STR$", 0xC1},
	{"CHR$", 0xC2},
	{"ATTR", 0xAB},
	{"BEEP", 0xD7},
	{"CODE", 0xAF},
	{"COPY", 0xFF},
	{"DATA", 0xE4},
	{"DRAW", 0xFC},
	{"LINE", 0xCA},
	{"LIST", 0xF0},
	{"LOAD", 0xEF},
	{"MOVE", 0xD1},
	{"NEXT", 0xF3},
	{"OVER", 0xDE},
	{"PEEK", 0xBE},
	{"PLOT", 0xF6},
	{"POKE", 0xF4},
	{"READ", 0xE3},
	{"SAVE", 0xF8},
	{"STEP", 0xCD},
	{"STOP", 0xE2},
	{"THEN", 0xCB},
	{"ABS", 0xBD},
	{"ACS", 0xB6},
	{"AND", 0xC6},
	{"ASN", 0xB5},
	{"ATN", 0xB7},
	{"BIN", 0xC4},
	{"CAT", 0xCF},
	{"CLS", 0xFB},
	{"COS", 0xB3},
	{"DIM", 0xE9},
	{"EXP", 0xB9},
	{"FOR", 0xEB},
	{"INK", 0xD9},
	{"INT", 0xBA},
	{"LEN", 0xB1},
	{"LET", 0xF1},
	{"NEW", 0xE6},
	{"NOT", 0xC3},
	{"OUT", 0xDF},
	{"REM", 0xEA},
	{"RND", 0xA5},
	{"RUN", 0xF7},
	{"SGN", 0xBC},
	{"SIN", 0xB2},
	{"SQR", 0xBB},
	{"TAB", 0xAD},
	{"TAN", 0xB4},
	{"USR", 0xC0},
	{"VAL", 0xB0},
	{"AT", 0xAC},
	{"FN", 0xA8},
	{"IF", 0xFA},
	{"IN", 0xBF},
	{"LN", 0xB8},
	{"OR", 0xC5},
	{"PI", 0xA7},
	{"TO", 0xCC},
	{"<=", 0xC7},
	{">=", 0xC8},
	{"<>", 0xC9},
}

// TokenizeBASIC converts a human-readable BASIC line into ZX Spectrum tokens.
//
// Input: "BORDER 0: LOAD \"\" CODE: RANDOMIZE USR 32768"
// Output: []byte{0xE7, ' ', '0', ':', ' ', 0xEF, '"', '"', ' ', 0xAF, ':', ...}
//
// Rules:
//   - Keywords are matched case-insensitively and replaced with token bytes
//   - String literals ("...") are passed through verbatim
//   - Numbers and punctuation are passed as ASCII
//   - Spaces between keywords and arguments are preserved
//   - After REM, everything is passed verbatim (no tokenization)
func TokenizeBASIC(line string) ([]byte, error) {
	var out []byte
	i := 0
	inREM := false

	for i < len(line) {
		// After REM, everything is literal
		if inREM {
			out = append(out, line[i])
			i++
			continue
		}

		// String literal: pass through verbatim
		if line[i] == '"' {
			out = append(out, '"')
			i++
			for i < len(line) && line[i] != '"' {
				out = append(out, line[i])
				i++
			}
			if i < len(line) {
				out = append(out, '"')
				i++
			}
			continue
		}

		// Try keyword match (case-insensitive, greedy)
		if matched, token, length := matchKeyword(line[i:]); matched {
			out = append(out, token)
			i += length
			if token == 0xEA { // REM
				inREM = true
			}
			continue
		}

		// Ordinary character (number, space, punctuation, etc.)
		out = append(out, line[i])
		i++
	}

	return out, nil
}

// matchKeyword tries to match a BASIC keyword at the start of s.
// Returns (matched, token, consumed_length).
func matchKeyword(s string) (bool, byte, int) {
	upper := strings.ToUpper(s)
	for _, kw := range basicKeywords {
		if strings.HasPrefix(upper, kw.keyword) {
			// Make sure we don't match a keyword prefix as part of a longer identifier.
			// E.g., "FORMULA" should not match "FOR" + "MULA".
			// Check: if keyword ends with a letter, the next char must not be a letter.
			kwLen := len(kw.keyword)
			if kwLen < len(s) {
				lastKW := kw.keyword[kwLen-1]
				nextCh := upper[kwLen]
				if isLetter(lastKW) && isLetter(nextCh) {
					continue // partial match, skip
				}
			}
			return true, kw.token, kwLen
		}
	}
	return false, 0, 0
}

func isLetter(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

// FormatTokenized returns a debug representation of tokenized BASIC.
func FormatTokenized(tokens []byte) string {
	var parts []string
	for _, b := range tokens {
		if b >= 0xA5 {
			// Find keyword name
			for _, kw := range basicKeywords {
				if kw.token == b {
					parts = append(parts, fmt.Sprintf("[%s]", kw.keyword))
					goto next
				}
			}
			parts = append(parts, fmt.Sprintf("[$%02X]", b))
		} else if b >= 0x20 && b < 0x7F {
			parts = append(parts, string(b))
		} else {
			parts = append(parts, fmt.Sprintf("\\x%02X", b))
		}
	next:
	}
	return strings.Join(parts, "")
}
