package c89

import (
	"regexp"
	"strings"
)

var cAngleHeaderRe = regexp.MustCompile(`#(?:[ \t]|/\*[^\r\n]*?\*/)*(?:include|include_next|import|embed)(?:[ \t]|/\*[^\r\n]*?\*/)*<[^>\r\n]*>`)

// cCodeMask identifies source bytes outside comments, literals and header names.
// Lexing uses the phase-2 view (escaped newlines removed) while replacements
// retain the original bytes and source lines. Apostrophes inside pp-numbers
// belong to the number; they are not character-literal delimiters.
func cCodeMask(src string) []bool {
	mask := make([]bool, len(src))
	for i := range mask {
		mask[i] = true
	}
	var logical []byte
	var offsets []int
	for i := 0; i < len(src); i++ {
		if src[i] == '\\' && i+1 < len(src) {
			if src[i+1] == '\n' {
				i++
				continue
			}
			if src[i+1] == '\r' && i+2 < len(src) && src[i+2] == '\n' {
				i += 2
				continue
			}
		}
		logical = append(logical, src[i])
		offsets = append(offsets, i)
	}
	protect := func(start, end int) {
		physicalEnd := len(src)
		if end < len(offsets) {
			physicalEnd = offsets[end]
		}
		for j := offsets[start]; j < physicalEnd; j++ {
			mask[j] = false
		}
	}
	// Recognize angle header tokens before quote scanning: an apostrophe in a
	// file name must not start a character literal spanning subsequent lines.
	headers := map[int]int{}
	for _, loc := range cAngleHeaderRe.FindAllIndex(logical, -1) {
		start := loc[0] + strings.IndexByte(string(logical[loc[0]:loc[1]]), '<')
		headers[start] = loc[1]
	}
	for i := 0; i < len(logical); {
		if end, ok := headers[i]; ok {
			protect(i, end)
			i = end
			continue
		}
		start := i
		switch {
		case logical[i] == '/' && i+1 < len(logical) && logical[i+1] == '/':
			i += 2
			for i < len(logical) && logical[i] != '\n' {
				i++
			}
			protect(start, i)
		case logical[i] == '/' && i+1 < len(logical) && logical[i+1] == '*':
			i += 2
			for i < len(logical) {
				if logical[i] == '*' && i+1 < len(logical) && logical[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			protect(start, i)
		case logical[i] == '"' || logical[i] == '\'':
			quote := logical[i]
			i++
			for i < len(logical) {
				if logical[i] == '\\' && i+1 < len(logical) {
					i += 2
					continue
				}
				i++
				if logical[i-1] == quote {
					break
				}
			}
			protect(start, i)
		case cppDigit(logical[i]) || logical[i] == '.' && i+1 < len(logical) && cppDigit(logical[i+1]):
			i++
			for i < len(logical) {
				c := logical[i]
				if cppDigit(c) || cppNondigit(c) || c == '.' || c == '\'' && i+1 < len(logical) && (cppDigit(logical[i+1]) || cppNondigit(logical[i+1])) || (c == '+' || c == '-') && strings.ContainsRune("eEpP", rune(logical[i-1])) {
					i++
					continue
				}
				break
			}
		default:
			i++
		}
	}
	return mask
}

func cppDigit(c byte) bool { return c >= '0' && c <= '9' }
func cppNondigit(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
}

// replaceCCodeMatches keeps the original spelling of every protected region.
// Attribute arguments may themselves contain a string, but their opening token
// must be code. '#embed' matches are anchored at the actual '#' token.
func replaceCCodeMatches(src string, pattern *regexp.Regexp, quotedArguments bool, replace func(string) string) string {
	locations := pattern.FindAllStringIndex(src, -1)
	if len(locations) == 0 {
		return src
	}
	mask := cCodeMask(src)
	var out strings.Builder
	last := 0
	for _, loc := range locations {
		anchor := loc[0]
		for anchor < loc[1] && (src[anchor] == ' ' || src[anchor] == '\t' || src[anchor] == '\n' || src[anchor] == '\r') {
			anchor++
		}
		if anchor == loc[1] || !mask[anchor] {
			continue
		}
		if !quotedArguments {
			allCode := true
			for _, code := range mask[loc[0]:loc[1]] {
				allCode = allCode && code
			}
			if !allCode {
				continue
			}
		}
		out.WriteString(src[last:loc[0]])
		match := src[loc[0]:loc[1]]
		replacement := replace(match)
		if removedLines := strings.Count(match, "\n") - strings.Count(replacement, "\n"); removedLines > 0 {
			out.WriteString(strings.Repeat("\n", removedLines))
		}
		out.WriteString(replacement)
		last = loc[1]
	}
	out.WriteString(src[last:])
	return out.String()
}
