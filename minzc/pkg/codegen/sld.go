package codegen

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/z80asm"
)

// SLDEntry represents a single source-level debug mapping
type SLDEntry struct {
	Address    int
	SourceFile string
	SourceLine int
	Size       int    // Instruction size in bytes
	Label      string // Optional label at this address
}

// SLDWriter generates SLD files for DeZog source-level debugging.
// SLD format: pipe-delimited text mapping Z80 addresses to source file/line.
type SLDWriter struct {
	entries []SLDEntry
}

// NewSLDWriter creates a new SLD writer.
func NewSLDWriter() *SLDWriter {
	return &SLDWriter{}
}

// AddEntry adds a source mapping entry.
func (w *SLDWriter) AddEntry(address int, sourceFile string, sourceLine int, size int) {
	w.entries = append(w.entries, SLDEntry{
		Address:    address,
		SourceFile: sourceFile,
		SourceLine: sourceLine,
		Size:       size,
	})
}

// AddLabel adds a label entry (shown in DeZog call stack).
func (w *SLDWriter) AddLabel(address int, label string) {
	w.entries = append(w.entries, SLDEntry{
		Address: address,
		Label:   label,
	})
}

// Write outputs the SLD file to the given path.
func (w *SLDWriter) Write(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create SLD file: %w", err)
	}
	defer f.Close()

	bw := bufio.NewWriter(f)
	defer bw.Flush()

	// SLD header
	fmt.Fprintln(bw, "|SLD.data.version|1")

	// Sort entries by address for clean output
	sort.Slice(w.entries, func(i, j int) bool {
		return w.entries[i].Address < w.entries[j].Address
	})

	// Emit label entries
	for _, e := range w.entries {
		if e.Label != "" {
			// Label definition: ||file|line|col|page|value|type|label
			fmt.Fprintf(bw, "||%s|0|0|0|%d|F|%s\n", "", e.Address, e.Label)
		}
	}

	// Emit source mapping entries
	for _, e := range w.entries {
		if e.SourceFile != "" && e.SourceLine > 0 {
			// Source mapping: ||file|line|col|page|address||size
			fmt.Fprintf(bw, "||%s|%d|0|0|%d||%d\n", e.SourceFile, e.SourceLine, e.Address, e.Size)
		}
	}

	return nil
}

// srcAnnotationRe matches "; @src:filename:line" comments in assembly output.
var srcAnnotationRe = regexp.MustCompile(`;\s*@src:([^:]+):(\d+)`)

// labelRe matches label definitions in assembly (word followed by colon).
var labelRe = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*):`)

// GenerateSLDFromAssembly builds SLD data from a .a80 file and assembly result.
// It parses @src annotations in the assembly comments to map addresses to MinZ source lines.
func GenerateSLDFromAssembly(asmFile string, result *z80asm.Result, sldFile string) error {
	// Parse the .a80 file for @src annotations
	f, err := os.Open(asmFile)
	if err != nil {
		return fmt.Errorf("failed to open assembly file: %w", err)
	}
	defer f.Close()

	// Build asm-line-number → (source-file, source-line) map from @src comments
	type srcInfo struct {
		file string
		line int
	}
	asmLineToSource := make(map[int]srcInfo)
	asmLineToLabel := make(map[int]string)

	scanner := bufio.NewScanner(f)
	asmLineNum := 0
	var currentSrc *srcInfo
	for scanner.Scan() {
		asmLineNum++
		line := scanner.Text()

		// Check for @src annotation
		if m := srcAnnotationRe.FindStringSubmatch(line); m != nil {
			lineNum, _ := strconv.Atoi(m[2])
			currentSrc = &srcInfo{file: m[1], line: lineNum}
		}

		// Check for label
		trimmed := strings.TrimSpace(line)
		if m := labelRe.FindStringSubmatch(trimmed); m != nil {
			asmLineToLabel[asmLineNum] = m[1]
		}

		// Associate this asm line with current source position
		if currentSrc != nil {
			asmLineToSource[asmLineNum] = *currentSrc
		}
	}

	// Build SLD from assembly listing
	writer := NewSLDWriter()

	// Add label entries from symbols
	for name, addr := range result.Symbols {
		// Skip internal/generated labels
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}
		writer.AddLabel(addr, name)
	}

	// Add source mapping entries from listing
	for _, listing := range result.Listing {
		if len(listing.Bytes) == 0 {
			continue
		}
		if src, ok := asmLineToSource[listing.LineNumber]; ok {
			writer.AddEntry(listing.Address, src.file, src.line, len(listing.Bytes))
		}
	}

	return writer.Write(sldFile)
}
