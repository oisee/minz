// ast-compare compares AST outputs between parsers
// Usage: ast-compare <corpus-dir> [new-parser-cmd]
//
// This tool runs regression tests by comparing the expected S-expression
// output with the output from a new parser implementation.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type TestPair struct {
	Name       string `json:"name"`
	SourcePath string `json:"source_path"`
	HasErrors  bool   `json:"has_errors"`
	ErrorNodes int    `json:"error_nodes"`
	NodeCount  int    `json:"node_count"`
}

type TestCorpus struct {
	Total      int        `json:"total"`
	Success    int        `json:"success"`
	WithErrors int        `json:"with_errors"`
	Tests      []TestPair `json:"tests"`
}

type CompareResult struct {
	Name           string
	Passed         bool
	ExpectedNodes  int
	ActualNodes    int
	ExpectedErrors int
	ActualErrors   int
	Diff           string
}

var (
	verbose     = flag.Bool("v", false, "Verbose output")
	skipErrors  = flag.Bool("skip-errors", false, "Skip tests that have parse errors")
	showDiff    = flag.Bool("diff", false, "Show diff for failed tests")
	maxFail     = flag.Int("max-fail", 10, "Maximum failures to show (0 = all)")
	parserCmd   = flag.String("parser", "", "Command to run new parser (receives file path, outputs S-expression)")
)

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("Usage: ast-compare [options] <corpus-dir>")
		fmt.Println("\nCompares AST outputs for parser regression testing")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  ast-compare tests/parser_corpus")
		fmt.Println("  ast-compare -parser './my-parser' tests/parser_corpus")
		fmt.Println("  ast-compare -skip-errors -diff tests/parser_corpus")
		os.Exit(1)
	}

	corpusDir := flag.Arg(0)

	// Read corpus metadata
	corpusFile := filepath.Join(corpusDir, "corpus.json")
	data, err := os.ReadFile(corpusFile)
	if err != nil {
		fmt.Printf("Error reading corpus: %v\n", err)
		os.Exit(1)
	}

	var corpus TestCorpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		fmt.Printf("Error parsing corpus: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Corpus: %d tests (%d clean, %d with errors)\n\n",
		corpus.Total, corpus.Success, corpus.WithErrors)

	if *parserCmd == "" {
		// Just validate the corpus structure
		validateCorpus(corpusDir, &corpus)
		return
	}

	// Run comparison tests
	runComparison(corpusDir, &corpus)
}

func validateCorpus(corpusDir string, corpus *TestCorpus) {
	fmt.Println("Validating corpus structure...\n")

	var missing, valid int
	for _, test := range corpus.Tests {
		testDir := filepath.Join(corpusDir, test.Name)
		inputFile := filepath.Join(testDir, "input.minz")
		sexpFile := filepath.Join(testDir, "expected.sexp")

		if _, err := os.Stat(inputFile); err != nil {
			if *verbose {
				fmt.Printf("Missing: %s/input.minz\n", test.Name)
			}
			missing++
			continue
		}

		if _, err := os.Stat(sexpFile); err != nil {
			if *verbose {
				fmt.Printf("Missing: %s/expected.sexp\n", test.Name)
			}
			missing++
			continue
		}

		valid++
	}

	fmt.Printf("Valid:   %d\n", valid)
	fmt.Printf("Missing: %d\n", missing)

	// Show breakdown by error status
	var cleanValid, errorValid int
	for _, test := range corpus.Tests {
		testDir := filepath.Join(corpusDir, test.Name)
		inputFile := filepath.Join(testDir, "input.minz")
		if _, err := os.Stat(inputFile); err == nil {
			if test.HasErrors {
				errorValid++
			} else {
				cleanValid++
			}
		}
	}

	fmt.Printf("\nBreakdown:\n")
	fmt.Printf("  Clean parses:      %d\n", cleanValid)
	fmt.Printf("  With parse errors: %d\n", errorValid)
}

func runComparison(corpusDir string, corpus *TestCorpus) {
	fmt.Printf("Running comparison with: %s\n\n", *parserCmd)

	var passed, failed, skipped int
	var failures []CompareResult

	for i, test := range corpus.Tests {
		if *skipErrors && test.HasErrors {
			skipped++
			continue
		}

		testDir := filepath.Join(corpusDir, test.Name)
		inputFile := filepath.Join(testDir, "input.minz")
		expectedFile := filepath.Join(testDir, "expected.sexp")

		// Read expected output
		expected, err := os.ReadFile(expectedFile)
		if err != nil {
			if *verbose {
				fmt.Printf("[%d] SKIP %s: %v\n", i+1, test.Name, err)
			}
			skipped++
			continue
		}

		// Run new parser
		cmd := exec.Command("sh", "-c", *parserCmd+" "+inputFile)
		actual, err := cmd.Output()
		if err != nil {
			if *verbose {
				fmt.Printf("[%d] FAIL %s: parser error: %v\n", i+1, test.Name, err)
			}
			failed++
			failures = append(failures, CompareResult{
				Name:   test.Name,
				Passed: false,
				Diff:   fmt.Sprintf("Parser error: %v", err),
			})
			continue
		}

		// Compare outputs
		expectedStr := strings.TrimSpace(string(expected))
		actualStr := strings.TrimSpace(string(actual))

		if expectedStr == actualStr {
			passed++
			if *verbose {
				fmt.Printf("[%d] PASS %s\n", i+1, test.Name)
			}
		} else {
			failed++
			result := CompareResult{
				Name:           test.Name,
				Passed:         false,
				ExpectedNodes:  test.NodeCount,
				ExpectedErrors: test.ErrorNodes,
				ActualNodes:    strings.Count(actualStr, "("),
				ActualErrors:   strings.Count(actualStr, "(ERROR"),
			}

			if *showDiff {
				result.Diff = generateDiff(expectedStr, actualStr)
			}

			failures = append(failures, result)

			if *verbose {
				fmt.Printf("[%d] FAIL %s (expected %d nodes, got %d)\n",
					i+1, test.Name, test.NodeCount, result.ActualNodes)
			}
		}
	}

	// Print summary
	fmt.Printf("\n=== Results ===\n")
	fmt.Printf("Passed:  %d\n", passed)
	fmt.Printf("Failed:  %d\n", failed)
	fmt.Printf("Skipped: %d\n", skipped)
	fmt.Printf("Total:   %d\n", passed+failed+skipped)

	if failed > 0 {
		fmt.Printf("\nSuccess rate: %.1f%%\n", float64(passed)*100/float64(passed+failed))
	}

	// Show failures
	if len(failures) > 0 && (*maxFail == 0 || len(failures) <= *maxFail) {
		fmt.Printf("\n=== Failures ===\n")
		for _, f := range failures {
			fmt.Printf("\n%s:\n", f.Name)
			if f.ExpectedNodes > 0 {
				fmt.Printf("  Expected: %d nodes, %d errors\n", f.ExpectedNodes, f.ExpectedErrors)
				fmt.Printf("  Actual:   %d nodes, %d errors\n", f.ActualNodes, f.ActualErrors)
			}
			if f.Diff != "" {
				fmt.Printf("  Diff:\n%s\n", indentLines(f.Diff, "    "))
			}
		}
	} else if len(failures) > *maxFail {
		fmt.Printf("\n(%d failures, showing first %d)\n", len(failures), *maxFail)
		for i := 0; i < *maxFail; i++ {
			f := failures[i]
			fmt.Printf("\n%s:\n", f.Name)
			fmt.Printf("  Expected: %d nodes, %d errors\n", f.ExpectedNodes, f.ExpectedErrors)
			fmt.Printf("  Actual:   %d nodes, %d errors\n", f.ActualNodes, f.ActualErrors)
		}
	}

	if failed > 0 {
		os.Exit(1)
	}
}

func generateDiff(expected, actual string) string {
	// Simple line-by-line comparison
	expLines := strings.Split(expected, "\n")
	actLines := strings.Split(actual, "\n")

	var diff strings.Builder
	maxLines := len(expLines)
	if len(actLines) > maxLines {
		maxLines = len(actLines)
	}

	diffCount := 0
	for i := 0; i < maxLines && diffCount < 10; i++ {
		var exp, act string
		if i < len(expLines) {
			exp = expLines[i]
		}
		if i < len(actLines) {
			act = actLines[i]
		}

		if exp != act {
			diffCount++
			if exp != "" {
				diff.WriteString(fmt.Sprintf("- %s\n", exp))
			}
			if act != "" {
				diff.WriteString(fmt.Sprintf("+ %s\n", act))
			}
		}
	}

	if diffCount >= 10 {
		diff.WriteString("... (truncated)\n")
	}

	return diff.String()
}

func indentLines(s, indent string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = indent + line
		}
	}
	return strings.Join(lines, "\n")
}
