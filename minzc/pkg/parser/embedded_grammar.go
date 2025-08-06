package parser

import (
	_ "embed"
	"os"
	"path/filepath"
	"io/ioutil"
	"strings"
)

// Embed the grammar.js file at compile time
// This makes mz self-contained and work from any directory
//go:embed grammar.js
var EmbeddedGrammarJS string

// Also embed the generated parser files
//go:embed grammar.json
var EmbeddedGrammarJSON string

//go:embed parser.c.txt
var EmbeddedParserC string

//go:embed node-types.json
var EmbeddedNodeTypes string

// SetupGrammar ensures grammar files exist in a temp directory for tree-sitter
func SetupGrammar() (string, error) {
	// Create a temp directory for the grammar
	tempDir, err := ioutil.TempDir("", "minz-grammar-")
	if err != nil {
		return "", err
	}
	
	// Write grammar.js to temp directory
	grammarPath := filepath.Join(tempDir, "grammar.js")
	if err := ioutil.WriteFile(grammarPath, []byte(EmbeddedGrammarJS), 0644); err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}
	
	// Create src directory
	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}
	
	// Write grammar.json
	if err := ioutil.WriteFile(filepath.Join(srcDir, "grammar.json"), []byte(EmbeddedGrammarJSON), 0644); err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}
	
	// Write parser.c with fixed includes
	// Replace the incorrect include path in parser.c
	fixedParserC := strings.Replace(EmbeddedParserC, `#include "tree_sitter/parser.h"`, `#include "../tree_sitter/parser.h"`, 1)
	if err := ioutil.WriteFile(filepath.Join(srcDir, "parser.c"), []byte(fixedParserC), 0644); err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}
	
	// Also create the tree_sitter directory with parser.h
	tsDir := filepath.Join(tempDir, "tree_sitter")
	if err := os.MkdirAll(tsDir, 0755); err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}
	
	// Write a minimal parser.h that tree-sitter needs
	parserH := `#ifndef TREE_SITTER_PARSER_H_
#define TREE_SITTER_PARSER_H_
typedef struct TSLanguage TSLanguage;
#endif`
	if err := ioutil.WriteFile(filepath.Join(tsDir, "parser.h"), []byte(parserH), 0644); err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}
	
	// Write node-types.json
	if err := ioutil.WriteFile(filepath.Join(srcDir, "node-types.json"), []byte(EmbeddedNodeTypes), 0644); err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}
	
	return tempDir, nil
}

// CleanupGrammar removes the temporary grammar directory
func CleanupGrammar(tempDir string) {
	if tempDir != "" && strings.HasPrefix(tempDir, os.TempDir()) {
		os.RemoveAll(tempDir)
	}
}

// ExtractGrammarTo extracts embedded grammar files to a specific directory
func ExtractGrammarTo(targetDir string) error {
	// Create the target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}
	
	// Write grammar.js
	grammarPath := filepath.Join(targetDir, "grammar.js")
	if err := ioutil.WriteFile(grammarPath, []byte(EmbeddedGrammarJS), 0644); err != nil {
		return err
	}
	
	// Create src directory
	srcDir := filepath.Join(targetDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return err
	}
	
	// Write grammar.json
	if err := ioutil.WriteFile(filepath.Join(srcDir, "grammar.json"), []byte(EmbeddedGrammarJSON), 0644); err != nil {
		return err
	}
	
	// Write parser.c with fixed includes
	fixedParserC := strings.Replace(EmbeddedParserC, `#include "tree_sitter/parser.h"`, `#include "../tree_sitter/parser.h"`, 1)
	if err := ioutil.WriteFile(filepath.Join(srcDir, "parser.c"), []byte(fixedParserC), 0644); err != nil {
		return err
	}
	
	// Create tree_sitter directory with parser.h
	tsDir := filepath.Join(targetDir, "tree_sitter")
	if err := os.MkdirAll(tsDir, 0755); err != nil {
		return err
	}
	
	// Write a minimal parser.h
	parserH := `#ifndef TREE_SITTER_PARSER_H_
#define TREE_SITTER_PARSER_H_
typedef struct TSLanguage TSLanguage;
#endif`
	if err := ioutil.WriteFile(filepath.Join(tsDir, "parser.h"), []byte(parserH), 0644); err != nil {
		return err
	}
	
	// Write node-types.json
	if err := ioutil.WriteFile(filepath.Join(srcDir, "node-types.json"), []byte(EmbeddedNodeTypes), 0644); err != nil {
		return err
	}
	
	return nil
}