package main

import (
	"fmt"
	"strings"
)

func main() {
	
	// Also test our unescaping
	testStrings := []string{
		`Hello\nWorld`,
		`Tab\there`,
		`Quote\"test`,
		`Back\\slash`,
	}
	
	for _, s := range testStrings {
		fmt.Printf("\nInput:  %q\n", s)
		fmt.Printf("Output: %q\n", unescapeString(s))
	}
}

func unescapeString(s string) string {
	var result []rune
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				result = append(result, '\n')
				i += 2
			case 't':
				result = append(result, '\t')
				i += 2
			case 'r':
				result = append(result, '\r')
				i += 2
			case '\\':
				result = append(result, '\\')
				i += 2
			case '"':
				result = append(result, '"')
				i += 2
			case '\'':
				result = append(result, '\'')
				i += 2
			case '0':
				result = append(result, '\x00')
				i += 2
			default:
				// Unknown escape, keep the backslash
				result = append(result, rune(s[i]))
				i++
			}
		} else {
			result = append(result, rune(s[i]))
			i++
		}
	}
	return string(result)
}