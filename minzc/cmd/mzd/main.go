// mzd - MinZ Z80 Disassembler
//
// Disassembles Z80 binary files with correct output for all instructions
// including undocumented opcodes and DDCB/FDCB prefixed operations.
//
// Usage:
//
//	mzd program.bin                    # Disassemble raw binary (ORG $0000)
//	mzd program.com                   # Auto-detect CP/M (ORG $0100)
//	mzd program.bin -o 0x8000         # Custom origin address
//	mzd program.bin --labels           # Auto-generate jump target labels
//	mzd program.bin --linear           # Force legacy linear disassembly
//	mzd program.bin --cycles           # Show T-state counts
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/disasm"
	"github.com/minz/minzc/pkg/disasm/analysis"
	"github.com/spf13/cobra"
)

var (
	orgAddr   string
	format    string
	startAddr string
	endAddr   string
	labels    bool
	hexDump   bool
	noAddr    bool

	// Analysis flags
	linear       bool
	target       string
	entryPoints  []string
	showStats    bool
	symFile      string
	exportSym    string
	reassemble   bool
	projectFile  string
	comments     []string
	markCode     []string
	markData     []string
	showCycles   bool
	noXrefs      bool
	noABI        bool
	abiFile      string
	exportABIFile string
)

// parseAddress parses hex (0x, $, suffix h), or decimal addresses.
func parseAddress(s string) (uint16, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty address")
	}

	// Handle $ prefix for hex (Z80 convention)
	if strings.HasPrefix(s, "$") {
		val, err := strconv.ParseUint(s[1:], 16, 16)
		return uint16(val), err
	}

	// Handle 0x prefix for hex
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		val, err := strconv.ParseUint(s[2:], 16, 16)
		return uint16(val), err
	}

	// Handle h suffix for hex
	if strings.HasSuffix(strings.ToLower(s), "h") {
		val, err := strconv.ParseUint(s[:len(s)-1], 16, 16)
		return uint16(val), err
	}

	// Decimal
	val, err := strconv.ParseUint(s, 10, 16)
	return uint16(val), err
}

// parseRange parses "XXXX-XXXX" hex range.
func parseRange(s string) (uint16, uint16, error) {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected XXXX-XXXX range, got %q", s)
	}
	start, err := parseAddress(parts[0])
	if err != nil {
		return 0, 0, err
	}
	end, err := parseAddress(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return start, end, nil
}

var rootCmd = &cobra.Command{
	Use:   "mzd <binary>",
	Short: "MinZ Z80 Disassembler",
	Long: `mzd - MinZ Z80 Disassembler
Disassembles Z80 binary files with correct output for all instructions
including undocumented opcodes and DDCB/FDCB prefixed operations.

File type auto-detection:
  .com  -> ORG $0100 (CP/M executable)
  .bin  -> ORG $0000 (raw binary)
  other -> ORG $0000 (raw binary)

Address formats:
  0x8000  - Hexadecimal (0x prefix)
  $8000   - Hexadecimal ($ prefix)
  8000h   - Hexadecimal (h suffix)
  32768   - Decimal

Output formats:
  annotated  - Address, hex bytes, mnemonic, branch comments (default)
  plain      - Mnemonic only
  hex        - Full hex dump with disassembly

Analysis (default):
  Recursive descent disassembly with code/data separation,
  cross-references, string detection, auto-labeling, and
  T-state cycle counting. Similar to IDA Pro.
  Use --linear to disable and get raw sequential disassembly.

Examples:
  mzd program.bin
  mzd program.com
  mzd program.bin -o 0x8000 --labels
  mzd program.bin -s 0x0010 -e 0x0080
  mzd program.bin --stats
  mzd program.bin -t cpm --cycles
  mzd program.bin --sym labels.sym -R
  mzd program.bin --linear`,
	Args: cobra.ExactArgs(1),
	RunE: runDisasm,
}

func init() {
	rootCmd.Flags().StringVarP(&orgAddr, "org", "o", "", "Origin address (auto-detect from extension if not set)")
	rootCmd.Flags().StringVarP(&format, "format", "f", "annotated", "Output format: plain, annotated, hex")
	rootCmd.Flags().StringVarP(&startAddr, "start", "s", "", "Start disassembly at address")
	rootCmd.Flags().StringVarP(&endAddr, "end", "e", "", "End disassembly at address")
	rootCmd.Flags().BoolVarP(&labels, "labels", "l", false, "Auto-generate labels for jump targets")
	rootCmd.Flags().BoolVar(&hexDump, "hex-dump", false, "Include hex dump column")
	rootCmd.Flags().BoolVar(&noAddr, "no-addr", false, "Hide address column")

	// Analysis flags
	rootCmd.Flags().BoolVar(&linear, "linear", false, "Force legacy linear disassembly (skip analysis)")
	// Keep --analyze as hidden alias for backwards compatibility
	rootCmd.Flags().Bool("analyze", false, "")
	rootCmd.Flags().MarkHidden("analyze")
	rootCmd.Flags().StringVarP(&target, "target", "t", "", "Platform: generic, cpm, spectrum, agon")
	rootCmd.Flags().StringArrayVar(&entryPoints, "entry", nil, "Additional entry point address (repeatable)")
	rootCmd.Flags().BoolVar(&showStats, "stats", false, "Print analysis statistics")
	rootCmd.Flags().StringVar(&symFile, "sym", "", "Load symbol file (.sym)")
	rootCmd.Flags().StringVar(&exportSym, "export-sym", "", "Export symbols to file")
	rootCmd.Flags().BoolVarP(&reassemble, "reassemble", "R", false, "Reassemblable output format")
	rootCmd.Flags().StringVarP(&projectFile, "project", "p", "", "Save/load analysis project (.mzp)")
	rootCmd.Flags().StringArrayVar(&comments, "comment", nil, "Add comment: XXXX=text (repeatable)")
	rootCmd.Flags().StringArrayVar(&markCode, "mark-code", nil, "Force range as code: XXXX-XXXX (repeatable)")
	rootCmd.Flags().StringArrayVar(&markData, "mark-data", nil, "Force range as data: XXXX-XXXX (repeatable)")
	rootCmd.Flags().BoolVar(&showCycles, "cycles", false, "Show T-state cycle counts")
	rootCmd.Flags().BoolVar(&noXrefs, "no-xrefs", false, "Suppress cross-reference comments")
	rootCmd.Flags().BoolVar(&noABI, "no-abi", false, "Suppress ABI/syscall annotations")
	rootCmd.Flags().StringVar(&abiFile, "abi", "", "Load additional .abi file (merged with built-in platform)")
	rootCmd.Flags().StringVar(&exportABIFile, "export-abi", "", "Export platform ABI profile to .abi file")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runDisasm(cmd *cobra.Command, args []string) error {
	inputFile := args[0]

	// Read binary file
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", inputFile, err)
	}

	if len(data) == 0 {
		return fmt.Errorf("file is empty: %s", inputFile)
	}

	// Determine origin address
	org := uint16(0x0000)
	if orgAddr != "" {
		org, err = parseAddress(orgAddr)
		if err != nil {
			return fmt.Errorf("invalid --org address %q: %w", orgAddr, err)
		}
	} else {
		// Auto-detect from file extension
		ext := strings.ToLower(filepath.Ext(inputFile))
		switch ext {
		case ".com":
			org = 0x0100
		}
	}

	// Linear mode is opt-in; analysis is the default
	if linear {
		return runLinear(data, org)
	}

	return runAnalyzed(inputFile, data, org)
}

func runLinear(data []byte, org uint16) error {
	var err error

	// Parse start/end addresses
	start := org
	if startAddr != "" {
		start, err = parseAddress(startAddr)
		if err != nil {
			return fmt.Errorf("invalid --start address %q: %w", startAddr, err)
		}
	}

	end := org + uint16(len(data))
	if int(org)+len(data) > 0xFFFF {
		end = 0xFFFF
	}
	if endAddr != "" {
		end, err = parseAddress(endAddr)
		if err != nil {
			return fmt.Errorf("invalid --end address %q: %w", endAddr, err)
		}
	}

	// Validate address range
	if start < org || start >= org+uint16(len(data)) {
		return fmt.Errorf("start address $%04X is outside file range $%04X-$%04X",
			start, org, org+uint16(len(data))-1)
	}

	// First pass: collect jump targets for label generation
	var jumpTargets map[uint16]string
	if labels {
		jumpTargets = collectJumpTargets(data, org, start, end)
	}

	// Second pass: disassemble and output
	pc := start
	for pc < end {
		offset := int(pc - org)
		if offset >= len(data) {
			break
		}

		remaining := data[offset:]
		if len(remaining) > 4 {
			remaining = remaining[:4]
		}

		mnemonic, size, targetAddr, relOffset := disasm.DisasmFull(remaining, pc)

		// Emit label if this address is a jump target
		if labels {
			if label, ok := jumpTargets[pc]; ok {
				fmt.Printf("%s:\n", label)
			}
		}

		// Build output line based on format
		switch format {
		case "plain":
			outputPlain(mnemonic, labels, jumpTargets, targetAddr, relOffset)
		case "hex":
			outputHex(pc, remaining[:size], mnemonic, labels, jumpTargets, targetAddr, relOffset)
		default: // "annotated"
			outputAnnotated(pc, remaining[:size], mnemonic, labels, jumpTargets, targetAddr, relOffset)
		}

		pc += uint16(size)
	}

	return nil
}

func runAnalyzed(inputFile string, data []byte, org uint16) error {
	var a *analysis.Analysis

	// Load existing project or create new analysis
	if projectFile != "" {
		if _, err := os.Stat(projectFile); err == nil {
			loaded, err := analysis.LoadProject(projectFile, data)
			if err != nil {
				return fmt.Errorf("failed to load project: %w", err)
			}
			a = loaded
		}
	}

	if a == nil {
		a = analysis.NewAnalysis(data, org)
	}

	// Apply user overrides
	for _, mc := range markCode {
		start, end, err := parseRange(mc)
		if err != nil {
			return fmt.Errorf("invalid --mark-code range: %w", err)
		}
		a.CodeOverrides[start] = end
	}
	for _, md := range markData {
		start, end, err := parseRange(md)
		if err != nil {
			return fmt.Errorf("invalid --mark-data range: %w", err)
		}
		a.DataOverrides[start] = end
	}

	// Apply comments
	for _, c := range comments {
		parts := strings.SplitN(c, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid --comment format, expected XXXX=text, got %q", c)
		}
		addr, err := parseAddress(parts[0])
		if err != nil {
			return fmt.Errorf("invalid comment address: %w", err)
		}
		a.Comments[addr] = parts[1]
	}

	// Detect platform and entry points
	platform := target
	if platform == "" {
		ext := strings.ToLower(filepath.Ext(inputFile))
		if ext == ".com" {
			platform = "cpm"
		} else {
			platform = "generic"
		}
	}
	a.DetectEntryPoints(platform)

	// Add manual entry points
	for _, ep := range entryPoints {
		addr, err := parseAddress(ep)
		if err != nil {
			return fmt.Errorf("invalid --entry address: %w", err)
		}
		a.AddEntryPoint(addr)
	}

	// Run analysis
	a.Analyze()

	// String and data detection
	a.DetectStrings(4)
	a.DetectDataBlocks()

	// Load symbols
	if platform != "" {
		a.LoadPlatformSymbols(platform)
	}
	if symFile != "" {
		if err := a.LoadSymbolFile(symFile); err != nil {
			return fmt.Errorf("failed to load symbol file: %w", err)
		}
	}

	// Auto-label
	a.AutoLabel()

	// ABI annotation (platform system call annotations)
	if !noABI {
		if abiFile != "" {
			userProfile, err := analysis.LoadABIFile(abiFile)
			if err != nil {
				return fmt.Errorf("failed to load ABI file %s: %w", abiFile, err)
			}
			builtinProfile := analysis.GetABIProfile(platform)
			if builtinProfile != nil {
				merged := analysis.MergeProfiles(builtinProfile, userProfile)
				a.Platform = platform
				a.AnnotateABIWithProfile(merged)
			} else {
				a.AnnotateABIWithProfile(userProfile)
			}
		} else {
			a.AnnotateABI()
		}
	}

	// Export ABI profile if requested
	if exportABIFile != "" {
		profile := analysis.GetABIProfile(platform)
		if profile == nil {
			return fmt.Errorf("no built-in ABI profile for platform %q", platform)
		}
		if err := analysis.ExportABIFile(profile, exportABIFile); err != nil {
			return fmt.Errorf("failed to export ABI file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Exported ABI profile to %s\n", exportABIFile)
	}

	// Recompute stats
	a.ComputeStats()

	// Save project if requested
	if projectFile != "" {
		if err := a.SaveProject(projectFile); err != nil {
			return fmt.Errorf("failed to save project: %w", err)
		}
	}

	// Export symbols if requested
	if exportSym != "" {
		if err := a.ExportSymbolFile(exportSym); err != nil {
			return fmt.Errorf("failed to export symbols: %w", err)
		}
	}

	// Render output
	if reassemble {
		renderReassemblable(a)
	} else {
		renderAnalyzed(a)
	}

	// Print stats
	if showStats {
		fmt.Fprintln(os.Stderr, a.StatsString())
	}

	return nil
}

func renderAnalyzed(a *analysis.Analysis) {
	endAddr := int(a.Origin) + len(a.Data)
	functionsSet := make(map[uint16]bool)
	for addr := range a.Functions {
		functionsSet[addr] = true
	}

	prevWasCode := false
	addr := int(a.Origin)

	for addr < endAddr && addr <= 0xFFFF {
		pc := uint16(addr)
		bc := a.ByteMap[pc]

		// Function header
		if functionsSet[pc] {
			if addr > int(a.Origin) {
				fmt.Println()
			}
			name := a.GetLabel(pc)
			if name == "" {
				name = fmt.Sprintf("sub_%04X", pc)
			}
			fmt.Printf("; ---- %s ($%04X) ----\n", name, pc)
		}

		// Xref comments
		if !noXrefs && bc == analysis.ByteCodeStart {
			if refs := a.GetXRefsTo(pc); len(refs) > 0 && !functionsSet[pc] {
				xrefStr := formatXRefs(a, refs)
				if xrefStr != "" {
					fmt.Printf("; XREF: %s\n", xrefStr)
				}
			}
		}

		// User comment
		if comment, ok := a.Comments[pc]; ok {
			fmt.Printf("; %s\n", comment)
		}

		// Label (non-function)
		if !functionsSet[pc] {
			if lbl := a.GetLabel(pc); lbl != "" {
				fmt.Printf("%s:\n", lbl)
			}
		}

		switch bc {
		case analysis.ByteCodeStart:
			if !prevWasCode && addr > int(a.Origin) && !functionsSet[pc] {
				fmt.Println() // Blank line before code block
			}
			prevWasCode = true

			mem := a.ReadBytes(pc, 4)
			mnemonic, size, targetAddr, _ := disasm.DisasmFull(mem, pc)

			// Replace target addresses with labels
			if targetAddr >= 0 {
				if lbl := a.GetLabel(uint16(targetAddr)); lbl != "" {
					mnemonic = replaceTargetWithLabel(mnemonic, targetAddr, lbl)
				}
			}

			// Format line
			var line strings.Builder
			if !noAddr {
				fmt.Fprintf(&line, "%04X: ", pc)
			}
			var hexBytes strings.Builder
			for i := 0; i < size; i++ {
				fmt.Fprintf(&hexBytes, "%02X ", mem[i])
			}
			fmt.Fprintf(&line, "%-12s%s", hexBytes.String(), mnemonic)

			// Cycle annotation
			if showCycles {
				taken, notTaken := analysis.Cycles(mem)
				padLen := 30 - len(mnemonic)
				if padLen < 2 {
					padLen = 2
				}
				if taken == notTaken {
					fmt.Fprintf(&line, "%s; %dT", strings.Repeat(" ", padLen), taken)
				} else {
					fmt.Fprintf(&line, "%s; %dT/%dT", strings.Repeat(" ", padLen), taken, notTaken)
				}
			}

			fmt.Println(line.String())
			addr += size

		case analysis.ByteCode:
			// Interior byte — skip (handled by CodeStart)
			addr++

		case analysis.ByteString:
			prevWasCode = false
			s, ok := a.Strings[pc]
			if ok {
				// Print string as DB "..."
				var line strings.Builder
				if !noAddr {
					fmt.Fprintf(&line, "%04X: ", pc)
				}
				fmt.Fprintf(&line, "%-12s", "")
				escaped := escapeString(s.Content)
				switch s.Terminator {
				case 0x00:
					fmt.Fprintf(&line, "DB \"%s\",0", escaped)
				case 0x0D:
					fmt.Fprintf(&line, "DB \"%s\",$0D", escaped)
				case 0x24:
					fmt.Fprintf(&line, "DB \"%s\",'$'", escaped)
				case 0x80:
					fmt.Fprintf(&line, "DB \"%s\" ; bit-7 terminated", escaped)
				default:
					fmt.Fprintf(&line, "DB \"%s\"", escaped)
				}
				fmt.Println(line.String())
				addr += s.Length
			} else {
				addr++
			}

		case analysis.ByteData:
			prevWasCode = false
			// Collect consecutive data bytes
			dataStart := addr
			var dataBytes []byte
			for addr < endAddr && addr <= 0xFFFF && a.ByteMap[uint16(addr)] == analysis.ByteData {
				b, _ := a.ReadByte(uint16(addr))
				dataBytes = append(dataBytes, b)
				addr++
			}
			renderDataBytes(uint16(dataStart), dataBytes)

		default: // ByteUndefined
			prevWasCode = false
			// Collect consecutive undefined bytes
			undefStart := addr
			var undefBytes []byte
			for addr < endAddr && addr <= 0xFFFF && a.ByteMap[uint16(addr)] == analysis.ByteUndefined {
				b, _ := a.ReadByte(uint16(addr))
				undefBytes = append(undefBytes, b)
				addr++
			}
			renderDataBytes(uint16(undefStart), undefBytes)
		}
	}
}

func renderReassemblable(a *analysis.Analysis) {
	fmt.Printf("    ORG $%04X\n\n", a.Origin)

	endAddr := int(a.Origin) + len(a.Data)
	functionsSet := make(map[uint16]bool)
	for addr := range a.Functions {
		functionsSet[addr] = true
	}

	addr := int(a.Origin)
	for addr < endAddr && addr <= 0xFFFF {
		pc := uint16(addr)
		bc := a.ByteMap[pc]

		// Function header
		if functionsSet[pc] {
			if addr > int(a.Origin) {
				fmt.Println()
			}
			name := a.GetLabel(pc)
			if name == "" {
				name = fmt.Sprintf("sub_%04X", pc)
			}
			fmt.Printf("; ---- %s ----\n", name)
		}

		// User comment
		if comment, ok := a.Comments[pc]; ok {
			fmt.Printf("; %s\n", comment)
		}

		// Label
		if lbl := a.GetLabel(pc); lbl != "" {
			fmt.Printf("%s:\n", lbl)
		}

		switch bc {
		case analysis.ByteCodeStart:
			mem := a.ReadBytes(pc, 4)
			mnemonic, size, targetAddr, _ := disasm.DisasmFull(mem, pc)

			// Replace target addresses with labels
			if targetAddr >= 0 {
				if lbl := a.GetLabel(uint16(targetAddr)); lbl != "" {
					mnemonic = replaceTargetWithLabel(mnemonic, targetAddr, lbl)
				}
			}

			fmt.Printf("    %s\n", mnemonic)
			addr += size

		case analysis.ByteCode:
			addr++

		case analysis.ByteString:
			s, ok := a.Strings[pc]
			if ok {
				escaped := escapeString(s.Content)
				switch s.Terminator {
				case 0x00:
					fmt.Printf("    DB \"%s\",0\n", escaped)
				case 0x0D:
					fmt.Printf("    DB \"%s\",$0D\n", escaped)
				case 0x24:
					fmt.Printf("    DB \"%s\",'$'\n", escaped)
				case 0x80:
					fmt.Printf("    DB \"%s\" ; bit-7 terminated\n", escaped)
				default:
					fmt.Printf("    DB \"%s\"\n", escaped)
				}
				addr += s.Length
			} else {
				addr++
			}

		case analysis.ByteData, analysis.ByteUndefined:
			start := addr
			var bytes []byte
			for addr < endAddr && addr <= 0xFFFF &&
				(a.ByteMap[uint16(addr)] == analysis.ByteData || a.ByteMap[uint16(addr)] == analysis.ByteUndefined) {
				b, _ := a.ReadByte(uint16(addr))
				bytes = append(bytes, b)
				addr++
			}
			_ = start
			// Emit as DB lines, 8 bytes per line
			for i := 0; i < len(bytes); i += 8 {
				end := i + 8
				if end > len(bytes) {
					end = len(bytes)
				}
				chunk := bytes[i:end]
				parts := make([]string, len(chunk))
				for j, b := range chunk {
					parts[j] = fmt.Sprintf("$%02X", b)
				}
				fmt.Printf("    DB %s\n", strings.Join(parts, ","))
			}
		}
	}
}

func renderDataBytes(startAddr uint16, bytes []byte) {
	if len(bytes) == 0 {
		return
	}

	// Emit DB lines, up to 8 bytes per line
	for i := 0; i < len(bytes); i += 8 {
		end := i + 8
		if end > len(bytes) {
			end = len(bytes)
		}
		chunk := bytes[i:end]

		var line strings.Builder
		if !noAddr {
			fmt.Fprintf(&line, "%04X: ", startAddr+uint16(i))
		}

		// Hex column
		var hexCol strings.Builder
		for _, b := range chunk {
			fmt.Fprintf(&hexCol, "%02X ", b)
		}
		fmt.Fprintf(&line, "%-12s", hexCol.String())

		// DB directive
		parts := make([]string, len(chunk))
		for j, b := range chunk {
			parts[j] = fmt.Sprintf("$%02X", b)
		}
		fmt.Fprintf(&line, "DB %s", strings.Join(parts, ","))

		fmt.Println(line.String())
	}
}

func formatXRefs(a *analysis.Analysis, refs []analysis.XRef) string {
	var parts []string
	for _, ref := range refs {
		label := a.FormatOperand(ref.From)
		switch ref.Type {
		case analysis.XRefCall:
			parts = append(parts, fmt.Sprintf("CALL from %s", label))
		case analysis.XRefJump:
			parts = append(parts, fmt.Sprintf("JP from %s", label))
		case analysis.XRefCondJump:
			parts = append(parts, fmt.Sprintf("JP cc from %s", label))
		case analysis.XRefRead:
			parts = append(parts, fmt.Sprintf("read from %s", label))
		case analysis.XRefWrite:
			parts = append(parts, fmt.Sprintf("write from %s", label))
		}
	}
	return strings.Join(parts, ", ")
}

func escapeString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// --- Legacy output functions ---

func outputPlain(mnemonic string, useLabels bool, targets map[uint16]string, targetAddr int, relOffset int) {
	if useLabels && targetAddr >= 0 {
		if label, ok := targets[uint16(targetAddr)]; ok {
			mnemonic = replaceTargetWithLabel(mnemonic, targetAddr, label)
		}
	}
	fmt.Println(mnemonic)
}

func outputAnnotated(pc uint16, bytes []byte, mnemonic string, useLabels bool, targets map[uint16]string, targetAddr int, relOffset int) {
	var line strings.Builder
	if !noAddr {
		fmt.Fprintf(&line, "%04X: ", pc)
	}

	var hexBytes strings.Builder
	for _, b := range bytes {
		fmt.Fprintf(&hexBytes, "%02X ", b)
	}
	fmt.Fprintf(&line, "%-12s", hexBytes.String())

	displayMnemonic := mnemonic
	if useLabels && targetAddr >= 0 {
		if label, ok := targets[uint16(targetAddr)]; ok {
			displayMnemonic = replaceTargetWithLabel(mnemonic, targetAddr, label)
		}
	}
	fmt.Fprintf(&line, "%s", displayMnemonic)

	comment := buildComment(useLabels, targets, targetAddr, relOffset)
	if comment != "" {
		padLen := 30 - len(displayMnemonic)
		if padLen < 2 {
			padLen = 2
		}
		fmt.Fprintf(&line, "%s; %s", strings.Repeat(" ", padLen), comment)
	}

	fmt.Println(line.String())
}

func outputHex(pc uint16, bytes []byte, mnemonic string, useLabels bool, targets map[uint16]string, targetAddr int, relOffset int) {
	var line strings.Builder
	if !noAddr {
		fmt.Fprintf(&line, "%04X: ", pc)
	}

	var hexBytes strings.Builder
	for _, b := range bytes {
		fmt.Fprintf(&hexBytes, "%02X ", b)
	}
	fmt.Fprintf(&line, "%-16s", hexBytes.String())

	fmt.Fprintf(&line, "|")
	for _, b := range bytes {
		if b >= 0x20 && b < 0x7F {
			line.WriteByte(b)
		} else {
			line.WriteByte('.')
		}
	}
	for i := len(bytes); i < 4; i++ {
		line.WriteByte(' ')
	}
	fmt.Fprintf(&line, "| ")

	displayMnemonic := mnemonic
	if useLabels && targetAddr >= 0 {
		if label, ok := targets[uint16(targetAddr)]; ok {
			displayMnemonic = replaceTargetWithLabel(mnemonic, targetAddr, label)
		}
	}
	fmt.Fprintf(&line, "%s", displayMnemonic)

	fmt.Println(line.String())
}

func buildComment(useLabels bool, targets map[uint16]string, targetAddr int, relOffset int) string {
	if targetAddr < 0 {
		return ""
	}

	var parts []string

	if useLabels {
		if _, ok := targets[uint16(targetAddr)]; ok {
			parts = append(parts, fmt.Sprintf("$%04X", targetAddr))
		}
	}

	if relOffset != 0 {
		if relOffset > 0 {
			parts = append(parts, fmt.Sprintf("jr +%d", relOffset))
		} else {
			parts = append(parts, fmt.Sprintf("jr %d", relOffset))
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

func replaceTargetWithLabel(mnemonic string, targetAddr int, label string) string {
	hexAddr := fmt.Sprintf("$%04X", targetAddr)
	return strings.Replace(mnemonic, hexAddr, label, 1)
}

// collectJumpTargets does a first pass over the binary to find all branch targets.
func collectJumpTargets(data []byte, org, start, end uint16) map[uint16]string {
	targets := make(map[uint16]int)

	pc := start
	for pc < end {
		offset := int(pc - org)
		if offset >= len(data) {
			break
		}

		remaining := data[offset:]
		if len(remaining) > 4 {
			remaining = remaining[:4]
		}

		_, size, targetAddr, _ := disasm.DisasmFull(remaining, pc)
		if targetAddr >= 0 {
			tgt := uint16(targetAddr)
			if tgt >= org && tgt < org+uint16(len(data)) {
				targets[tgt]++
			}
		}
		pc += uint16(size)
	}

	result := make(map[uint16]string)

	addrs := make([]uint16, 0, len(targets))
	for addr := range targets {
		addrs = append(addrs, addr)
	}
	sort.Slice(addrs, func(i, j int) bool { return addrs[i] < addrs[j] })

	for _, addr := range addrs {
		result[addr] = fmt.Sprintf("L%04X", addr)
	}

	return result
}
