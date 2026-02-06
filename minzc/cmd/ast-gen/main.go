// ast-gen generates AST test pairs from MinZ files using tree-sitter
// Usage: ast-gen [options] <output-dir>
//
// This creates golden test files for parser regression testing.
// Each test pair consists of:
//   - input.minz: the original MinZ source
//   - expected.sexp: the tree-sitter S-expression AST
//   - metadata.json: file info and parse status

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type TestPair struct {
	Name       string    `json:"name"`
	SourcePath string    `json:"source_path"`
	SourceHash string    `json:"source_hash"`
	HasErrors  bool      `json:"has_errors"`
	ErrorNodes int       `json:"error_nodes"`
	NodeCount  int       `json:"node_count"`
	Generated  time.Time `json:"generated"`
}

type TestCorpus struct {
	Version   string     `json:"version"`
	Generated time.Time  `json:"generated"`
	Total     int        `json:"total"`
	Success   int        `json:"success"`
	WithErrors int       `json:"with_errors"`
	Failed    int        `json:"failed"`
	Tests     []TestPair `json:"tests"`
}

var (
	grammarDir  = flag.String("grammar", "", "Path to tree-sitter grammar directory")
	maxFiles    = flag.Int("max", 0, "Maximum number of files to process (0 = all)")
	verbose     = flag.Bool("v", false, "Verbose output")
	excludeDirs = flag.String("exclude", "_archive,node_modules,vendor", "Comma-separated directories to exclude")
)

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("Usage: ast-gen [options] <output-dir>")
		fmt.Println("\nGenerates AST test pairs from MinZ files")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	outputDir := flag.Arg(0)

	// Find grammar directory (where grammar.js lives)
	if *grammarDir == "" {
		// Try to find it relative to current dir
		candidates := []string{
			".",
			"../",
			"../../",
			filepath.Join(os.Getenv("HOME"), "dev/minz"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(filepath.Join(c, "grammar.js")); err == nil {
				*grammarDir = c
				break
			}
		}
		if *grammarDir == "" {
			fmt.Println("Error: Could not find tree-sitter grammar directory (containing grammar.js)")
			fmt.Println("Use -grammar flag to specify it")
			os.Exit(1)
		}
	}
	fmt.Printf("Using grammar directory: %s\n", *grammarDir)

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Find all .minz files
	excludeList := strings.Split(*excludeDirs, ",")
	var minzFiles []string

	searchDirs := []string{"."}
	if flag.NArg() > 1 {
		searchDirs = flag.Args()[1:]
	}

	for _, searchDir := range searchDirs {
		err := filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			// Skip excluded directories
			if info.IsDir() {
				for _, exc := range excludeList {
					if info.Name() == exc || strings.Contains(path, "/"+exc+"/") {
						return filepath.SkipDir
					}
				}
				return nil
			}

			// Only .minz files
			if strings.HasSuffix(path, ".minz") {
				minzFiles = append(minzFiles, path)
			}
			return nil
		})
		if err != nil {
			fmt.Printf("Warning: Error walking %s: %v\n", searchDir, err)
		}
	}

	// Sort for consistent ordering
	sort.Strings(minzFiles)

	// Limit if requested
	if *maxFiles > 0 && len(minzFiles) > *maxFiles {
		minzFiles = minzFiles[:*maxFiles]
	}

	fmt.Printf("Found %d MinZ files\n", len(minzFiles))

	// Process each file
	corpus := TestCorpus{
		Version:   "1.0",
		Generated: time.Now(),
		Tests:     make([]TestPair, 0, len(minzFiles)),
	}

	for i, file := range minzFiles {
		if *verbose {
			fmt.Printf("[%d/%d] Processing %s\n", i+1, len(minzFiles), file)
		} else if (i+1)%10 == 0 {
			fmt.Printf("\rProcessing... %d/%d", i+1, len(minzFiles))
		}

		pair, err := processFile(file, outputDir)
		if err != nil {
			if *verbose {
				fmt.Printf("  Error: %v\n", err)
			}
			corpus.Failed++
			continue
		}

		corpus.Tests = append(corpus.Tests, *pair)
		if pair.HasErrors {
			corpus.WithErrors++
		} else {
			corpus.Success++
		}
	}

	fmt.Printf("\rProcessed %d files\n", len(minzFiles))

	corpus.Total = len(corpus.Tests)

	// Write corpus metadata
	corpusFile := filepath.Join(outputDir, "corpus.json")
	corpusData, _ := json.MarshalIndent(corpus, "", "  ")
	if err := os.WriteFile(corpusFile, corpusData, 0644); err != nil {
		fmt.Printf("Error writing corpus file: %v\n", err)
	}

	// Print summary
	fmt.Println("\n=== Summary ===")
	fmt.Printf("Total:       %d\n", corpus.Total)
	fmt.Printf("Success:     %d (%.1f%%)\n", corpus.Success, float64(corpus.Success)*100/float64(corpus.Total))
	fmt.Printf("With errors: %d (%.1f%%)\n", corpus.WithErrors, float64(corpus.WithErrors)*100/float64(corpus.Total))
	fmt.Printf("Failed:      %d\n", corpus.Failed)
	fmt.Printf("\nOutput: %s\n", outputDir)
}

func processFile(sourcePath, outputDir string) (*TestPair, error) {
	// Read source file
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("read source: %w", err)
	}

	// Calculate hash for deduplication
	hash := sha256.Sum256(source)
	hashStr := hex.EncodeToString(hash[:8])

	// Create safe name from path
	safeName := strings.ReplaceAll(sourcePath, "/", "_")
	safeName = strings.ReplaceAll(safeName, "\\", "_")
	safeName = strings.TrimPrefix(safeName, "._")
	safeName = strings.TrimSuffix(safeName, ".minz")

	// Create test directory
	testDir := filepath.Join(outputDir, safeName)
	if err := os.MkdirAll(testDir, 0755); err != nil {
		return nil, fmt.Errorf("create test dir: %w", err)
	}

	// Write source file
	inputFile := filepath.Join(testDir, "input.minz")
	if err := os.WriteFile(inputFile, source, 0644); err != nil {
		return nil, fmt.Errorf("write input: %w", err)
	}

	// Run tree-sitter to get S-expression
	// tree-sitter must be run from the grammar directory
	absPath, _ := filepath.Abs(sourcePath)
	grammarAbs, _ := filepath.Abs(*grammarDir)

	// Check if tree-sitter-minz exists in grammar directory
	cmd := exec.Command("tree-sitter", "parse", absPath)
	cmd.Dir = grammarAbs
	cmd.Env = append(os.Environ(), "NO_COLOR=1")

	output, _ := cmd.CombinedOutput()

	// Clean up the output
	sexp := cleanSExp(string(output))

	// Write S-expression
	sexpFile := filepath.Join(testDir, "expected.sexp")
	if err := os.WriteFile(sexpFile, []byte(sexp), 0644); err != nil {
		return nil, fmt.Errorf("write sexp: %w", err)
	}

	// Count ERROR nodes
	errorCount := strings.Count(sexp, "(ERROR")
	nodeCount := strings.Count(sexp, "(")

	// Create metadata
	pair := &TestPair{
		Name:       safeName,
		SourcePath: sourcePath,
		SourceHash: hashStr,
		HasErrors:  errorCount > 0,
		ErrorNodes: errorCount,
		NodeCount:  nodeCount,
		Generated:  time.Now(),
	}

	metaFile := filepath.Join(testDir, "metadata.json")
	metaData, _ := json.MarshalIndent(pair, "", "  ")
	if err := os.WriteFile(metaFile, metaData, 0644); err != nil {
		return nil, fmt.Errorf("write metadata: %w", err)
	}

	return pair, nil
}

func cleanSExp(output string) string {
	// Strip ANSI escape codes
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	output = ansiRegex.ReplaceAllString(output, "")

	// Remove statistics and warning lines
	lines := strings.Split(output, "\n")
	var sexpLines []string
	for _, line := range lines {
		if line == "" {
			continue
		}
		if strings.Contains(line, "\tParse:") {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Warning:") ||
			strings.HasPrefix(trimmed, "Error:") ||
			strings.HasPrefix(trimmed, "No ") {
			continue
		}
		sexpLines = append(sexpLines, line)
	}

	return strings.Join(sexpLines, "\n")
}
