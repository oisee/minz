package analysis

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// LoadABIFile parses a .abi file and returns an ABIProfile.
func LoadABIFile(path string) (*ABIProfile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseABI(f)
}

// ParseABI parses .abi format from a reader.
func ParseABI(r io.Reader) (*ABIProfile, error) {
	profile := &ABIProfile{}
	scanner := bufio.NewScanner(r)

	var currentEntry *SyscallEntry
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || line[0] == ';' || line[0] == '#' {
			continue
		}

		// Section header
		if line[0] == '[' {
			idx := strings.Index(line, "]")
			if idx < 0 {
				return nil, fmt.Errorf("line %d: unclosed section bracket", lineNum)
			}
			section := line[1:idx]

			if section == "platform" {
				currentEntry = nil
				continue
			}

			entry, err := parseEntrySection(section, lineNum)
			if err != nil {
				return nil, err
			}
			currentEntry = entry
			profile.Entries = append(profile.Entries, currentEntry)
			continue
		}

		// Key=Value line
		eqIdx := strings.Index(line, "=")
		if eqIdx < 0 {
			return nil, fmt.Errorf("line %d: expected key=value, got %q", lineNum, line)
		}
		key := strings.TrimSpace(line[:eqIdx])
		value := strings.TrimSpace(line[eqIdx+1:])

		// Platform section keys
		if currentEntry == nil {
			switch key {
			case "name":
				profile.Platform = value
			case "arch":
				// stored as metadata, no field currently
			default:
				// ignore unknown platform keys
			}
			continue
		}

		// Entry section keys
		switch key {
		case "name":
			currentEntry.Name = value
		case "desc":
			currentEntry.DirectDesc = value
		case "dispatch":
			currentEntry.DispatchReg = value
			if currentEntry.Functions == nil {
				currentEntry.Functions = make(map[int]*SyscallDef)
			}
		case "params":
			currentEntry.DirectParams = parseParamList(value)
		default:
			// Numeric key = dispatched function definition
			num, err := strconv.Atoi(key)
			if err != nil {
				return nil, fmt.Errorf("line %d: unknown key %q", lineNum, key)
			}
			def, err := parseDispatchedDef(num, value, lineNum)
			if err != nil {
				return nil, err
			}
			if currentEntry.Functions == nil {
				currentEntry.Functions = make(map[int]*SyscallDef)
			}
			currentEntry.Functions[num] = def
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return profile, nil
}

// parseEntrySection parses "[entry:CALL:0x0005]" or "[entry:RST:0x10]".
func parseEntrySection(section string, lineNum int) (*SyscallEntry, error) {
	parts := strings.SplitN(section, ":", 3)
	if len(parts) != 3 || parts[0] != "entry" {
		return nil, fmt.Errorf("line %d: invalid section %q, expected [entry:CALL:0xNNNN]", lineNum, section)
	}
	trigger := parts[1]
	if trigger != "CALL" && trigger != "RST" {
		return nil, fmt.Errorf("line %d: invalid trigger %q, expected CALL or RST", lineNum, trigger)
	}
	addr, err := parseHexAddress(parts[2])
	if err != nil {
		return nil, fmt.Errorf("line %d: invalid address %q: %w", lineNum, parts[2], err)
	}
	return &SyscallEntry{
		Trigger: trigger,
		Address: addr,
	}, nil
}

// parseHexAddress parses "0x0005", "0x10", "$0DAF", etc.
func parseHexAddress(s string) (uint16, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		val, err := strconv.ParseUint(s[2:], 16, 16)
		return uint16(val), err
	}
	if strings.HasPrefix(s, "$") {
		val, err := strconv.ParseUint(s[1:], 16, 16)
		return uint16(val), err
	}
	// Try hex without prefix
	val, err := strconv.ParseUint(s, 16, 16)
	return uint16(val), err
}

// parseDispatchedDef parses "CONSOLE_OUTPUT,Write char to console,E:char" or
// "SYSTEM_RESET,Warm boot (terminate),noreturn".
func parseDispatchedDef(num int, value string, lineNum int) (*SyscallDef, error) {
	parts := splitCSV(value)
	if len(parts) < 1 {
		return nil, fmt.Errorf("line %d: empty function definition", lineNum)
	}

	def := &SyscallDef{
		Number: num,
		Name:   parts[0],
	}

	if len(parts) >= 2 {
		def.Desc = parts[1]
	}

	// Remaining parts: either "noreturn" or "Reg:Name" params
	for i := 2; i < len(parts); i++ {
		p := strings.TrimSpace(parts[i])
		if p == "noreturn" {
			def.NoReturn = true
			continue
		}
		param := parseParam(p)
		if param.Reg != "" {
			def.Params = append(def.Params, param)
		}
	}

	return def, nil
}

// parseParamList parses "A:char" or "HL:addr,A:data".
func parseParamList(value string) []SyscallParam {
	parts := splitCSV(value)
	var params []SyscallParam
	for _, p := range parts {
		param := parseParam(strings.TrimSpace(p))
		if param.Reg != "" {
			params = append(params, param)
		}
	}
	return params
}

// parseParam parses "E:char" or "DE:addr".
func parseParam(s string) SyscallParam {
	idx := strings.Index(s, ":")
	if idx < 0 {
		return SyscallParam{}
	}
	return SyscallParam{
		Reg:  strings.TrimSpace(s[:idx]),
		Name: strings.TrimSpace(s[idx+1:]),
	}
}

// splitCSV splits on commas, but is simple (no quoting).
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// ExportABIFile writes an ABIProfile to a .abi file.
func ExportABIFile(profile *ABIProfile, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return WriteABI(profile, f)
}

// WriteABI writes an ABIProfile in .abi format to a writer.
func WriteABI(profile *ABIProfile, w io.Writer) error {
	bw := bufio.NewWriter(w)

	fmt.Fprintf(bw, "; mzd ABI definition file\n")
	fmt.Fprintf(bw, "; Format version 1\n\n")

	fmt.Fprintf(bw, "[platform]\n")
	fmt.Fprintf(bw, "name=%s\n", profile.Platform)
	fmt.Fprintf(bw, "arch=z80\n\n")

	for _, entry := range profile.Entries {
		fmt.Fprintf(bw, "[entry:%s:0x%04X]\n", entry.Trigger, entry.Address)
		if entry.Name != "" {
			fmt.Fprintf(bw, "name=%s\n", entry.Name)
		}

		if entry.DispatchReg != "" {
			// Dispatched entry
			fmt.Fprintf(bw, "dispatch=%s\n", entry.DispatchReg)

			// Sort function numbers for deterministic output
			nums := make([]int, 0, len(entry.Functions))
			for n := range entry.Functions {
				nums = append(nums, n)
			}
			sort.Ints(nums)

			for _, n := range nums {
				def := entry.Functions[n]
				line := def.Name
				if def.Desc != "" {
					line += "," + def.Desc
				}
				for _, p := range def.Params {
					line += "," + p.Reg + ":" + p.Name
				}
				if def.NoReturn {
					line += ",noreturn"
				}
				fmt.Fprintf(bw, "%d=%s\n", n, line)
			}
		} else {
			// Direct entry
			if entry.DirectDesc != "" {
				fmt.Fprintf(bw, "desc=%s\n", entry.DirectDesc)
			}
			if len(entry.DirectParams) > 0 {
				var parts []string
				for _, p := range entry.DirectParams {
					parts = append(parts, p.Reg+":"+p.Name)
				}
				fmt.Fprintf(bw, "params=%s\n", strings.Join(parts, ","))
			}
		}
		fmt.Fprintln(bw)
	}

	return bw.Flush()
}

// MergeProfiles creates a new profile by overlaying entries from overlay onto base.
// Entries at the same (Trigger, Address) pair are replaced; new entries are appended.
func MergeProfiles(base, overlay *ABIProfile) *ABIProfile {
	result := &ABIProfile{
		Platform: overlay.Platform,
	}
	if result.Platform == "" {
		result.Platform = base.Platform
	}

	// Index base entries by (trigger, address)
	type entryKey struct {
		trigger string
		addr    uint16
	}
	baseMap := make(map[entryKey]*SyscallEntry)
	var baseOrder []entryKey
	for _, e := range base.Entries {
		k := entryKey{e.Trigger, e.Address}
		baseMap[k] = e
		baseOrder = append(baseOrder, k)
	}

	// Track which base entries are replaced
	replaced := make(map[entryKey]bool)

	// Apply overlay
	overlayMap := make(map[entryKey]*SyscallEntry)
	var overlayOrder []entryKey
	for _, e := range overlay.Entries {
		k := entryKey{e.Trigger, e.Address}
		overlayMap[k] = e
		if _, exists := baseMap[k]; exists {
			replaced[k] = true
		}
		overlayOrder = append(overlayOrder, k)
	}

	// Add base entries (skip replaced ones)
	for _, k := range baseOrder {
		if replaced[k] {
			continue
		}
		result.Entries = append(result.Entries, baseMap[k])
	}

	// Add all overlay entries
	for _, k := range overlayOrder {
		result.Entries = append(result.Entries, overlayMap[k])
	}

	return result
}
