package main

import (
	"fmt"
	"strings"
)

// isLocalLabel checks if a label is a local label (starts with exactly one dot)
func isLocalLabel(label string) bool {
	return strings.HasPrefix(label, ".") && !strings.HasPrefix(label, "..")
}

func main() {
	testLabels := []string{
		".loop",
		"..double",
		"...games.snake.SCREEN_WIDTH",
		"main",
		".test",
	}
	
	for _, label := range testLabels {
		fmt.Printf("Label '%s' is local: %v\n", label, isLocalLabel(label))
	}
}