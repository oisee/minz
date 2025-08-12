package parser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// GrammarManager handles grammar file management and tree-sitter installation
type GrammarManager struct {
	grammarPath string
}

// NewGrammarManager creates a new grammar manager
func NewGrammarManager() *GrammarManager {
	return &GrammarManager{}
}

// FindGrammarPath locates the grammar files
func (g *GrammarManager) FindGrammarPath() (string, error) {
	// Check multiple locations for grammar.js
	locations := []string{
		// Next to binary
		filepath.Join(filepath.Dir(os.Args[0]), "grammar.js"),
		// Current directory
		"grammar.js",
		// Parent directory (development)
		"../grammar.js",
		// Two levels up
		"../../grammar.js",
		// Home directory
		filepath.Join(os.Getenv("HOME"), ".minz", "grammar", "grammar.js"),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			g.grammarPath = filepath.Dir(loc)
			return g.grammarPath, nil
		}
	}

	return "", fmt.Errorf("grammar.js not found in any expected location")
}

// EnsureTreeSitter checks if tree-sitter is available
func (g *GrammarManager) EnsureTreeSitter() error {
	if _, err := exec.LookPath("tree-sitter"); err != nil {
		return fmt.Errorf(`tree-sitter CLI is required but not installed.

Please install tree-sitter:

macOS:
  brew install tree-sitter

Ubuntu/Debian:
  npm install -g tree-sitter-cli

Or download from:
  https://github.com/tree-sitter/tree-sitter/releases`)
	}
	return nil
}
