package analysis

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// DeclaredFunc holds the compiler-declared ABI for one function.
type DeclaredFunc struct {
	Name    string
	In      RegSet // parameter registers
	Out     RegSet // return registers
	Clobber RegSet // declared clobbers
	Line    int    // source line in .a80
	Raw     string // original comment text
}

// ParseAsmABI extracts declared function ABIs from compiler-generated .a80 comments.
// Format: ; fun name(param: type = REG, ...) -> type = REG ; clobbers: REG, ...
// Also parses "name:" labels following each ABI comment for name→label matching.
func ParseAsmABI(path string) ([]*DeclaredFunc, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var funcs []*DeclaredFunc
	scanner := bufio.NewScanner(f)
	lineNo := 0
	var pendingFunc *DeclaredFunc

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())

		// If we just parsed a "; fun ..." comment, the next non-empty
		// non-comment line should be the "name:" label — verify they match.
		if pendingFunc != nil && line != "" && !strings.HasPrefix(line, ";") {
			// Check if it's "label:" matching the function name
			if colonIdx := strings.IndexByte(line, ':'); colonIdx > 0 {
				label := strings.TrimSpace(line[:colonIdx])
				if label != pendingFunc.Name {
					// Label doesn't match — might be a sub-label, keep pending
					pendingFunc.Name = label // use the actual label name
				}
			}
			pendingFunc = nil
		}

		if !strings.HasPrefix(line, "; fun ") {
			continue
		}

		df := parseFuncComment(line[2:]) // skip "; "
		if df != nil {
			df.Line = lineNo
			df.Raw = line
			funcs = append(funcs, df)
			pendingFunc = df
		}
	}

	return funcs, scanner.Err()
}

// BuildFuncMap creates a name→address mapping by matching declared function names
// to labels/functions detected in the analysis.
func BuildFuncMap(declared []*DeclaredFunc, a *Analysis) map[string]uint16 {
	funcMap := make(map[string]uint16)

	// First: check labels in analysis
	for addr, lbl := range a.Labels {
		for _, df := range declared {
			if lbl.Name == df.Name {
				funcMap[df.Name] = addr
			}
		}
	}

	// Also check function names
	for _, fn := range a.Functions {
		for _, df := range declared {
			if fn.Name == df.Name {
				funcMap[df.Name] = fn.Entry
			}
		}
	}

	return funcMap
}

// regPattern matches register names in ABI comments.
var regPattern = regexp.MustCompile(`\b(A|F|B|C|D|E|H|L|AF|BC|DE|HL|IX|IY|IXH|IXL|IYH|IYL)\b`)

// parseFuncComment parses "fun name(...) -> type = REG ; clobbers: ..."
func parseFuncComment(s string) *DeclaredFunc {
	if !strings.HasPrefix(s, "fun ") {
		return nil
	}

	df := &DeclaredFunc{}

	// Extract function name
	rest := s[4:]
	parenIdx := strings.IndexByte(rest, '(')
	if parenIdx < 0 {
		return nil
	}
	df.Name = strings.TrimSpace(rest[:parenIdx])

	// Split at "; clobbers:" to separate params/return from clobbers
	mainPart := rest
	clobberPart := ""
	if idx := strings.Index(rest, "; clobbers:"); idx >= 0 {
		mainPart = rest[:idx]
		clobberPart = rest[idx+len("; clobbers:"):]
	}

	// Extract parameter registers from (param: type = REG, ...)
	closeIdx := strings.IndexByte(mainPart, ')')
	if closeIdx < 0 {
		return nil
	}
	paramStr := mainPart[parenIdx+1 : closeIdx]
	df.In = extractRegsFromParams(paramStr)

	// Extract return register from "-> type = REG"
	afterParams := mainPart[closeIdx+1:]
	if arrowIdx := strings.Index(afterParams, "->"); arrowIdx >= 0 {
		retPart := afterParams[arrowIdx+2:]
		df.Out = extractReturnReg(retPart)
	}

	// Extract clobber registers
	if clobberPart != "" {
		df.Clobber = extractClobberRegs(clobberPart)
	}

	return df
}

// extractRegsFromParams extracts register assignments from parameter list.
// Input: "addr: u16 = HL, val: u8 = C"
func extractRegsFromParams(s string) RegSet {
	var result RegSet
	params := strings.Split(s, ",")
	for _, p := range params {
		p = strings.TrimSpace(p)
		if eqIdx := strings.LastIndex(p, "= "); eqIdx >= 0 {
			regStr := strings.TrimSpace(p[eqIdx+2:])
			result |= parseRegName(regStr)
		}
	}
	return result
}

// extractReturnReg extracts the return register from "type = REG" or "(type = REG, type = REG)"
func extractReturnReg(s string) RegSet {
	var result RegSet
	// Handle multi-return: (u8 = A, u16 = HL)
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "(") {
		s = strings.Trim(s, "()")
		parts := strings.Split(s, ",")
		for _, p := range parts {
			if eqIdx := strings.LastIndex(p, "= "); eqIdx >= 0 {
				result |= parseRegName(strings.TrimSpace(p[eqIdx+2:]))
			}
		}
		return result
	}
	// Single return: type = REG
	if eqIdx := strings.LastIndex(s, "= "); eqIdx >= 0 {
		result |= parseRegName(strings.TrimSpace(s[eqIdx+2:]))
	}
	return result
}

// extractClobberRegs parses "A, BC, F, HL, mem, stack" → RegSet (ignoring mem/stack).
func extractClobberRegs(s string) RegSet {
	var result RegSet
	parts := strings.Split(s, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		result |= parseRegName(p)
	}
	return result
}

// parseRegName converts a register name string to RegSet.
func parseRegName(name string) RegSet {
	switch name {
	case "A":
		return RegA
	case "F":
		return RegF
	case "B":
		return RegB
	case "C":
		return RegC
	case "D":
		return RegD
	case "E":
		return RegE
	case "H":
		return RegH
	case "L":
		return RegL
	case "AF":
		return RegAF
	case "BC":
		return RegBC
	case "DE":
		return RegDE
	case "HL":
		return RegHL
	// IX/IY not tracked in our RegSet, ignore silently
	case "IX", "IY", "IXH", "IXL", "IYH", "IYL":
		return 0
	// Non-register annotations (mem, stack) — ignore
	default:
		return 0
	}
}

// ABIMismatch describes a difference between declared and detected ABI.
type ABIMismatch struct {
	FuncName string
	Addr     uint16 // function address in binary (0 if not resolved)
	Field    string // "IN", "OUT", "CLOBBER"
	Declared RegSet
	Detected RegSet
	Extra    RegSet // in detected but not declared
	Missing  RegSet // in declared but not detected
}

func (m *ABIMismatch) String() string {
	parts := []string{fmt.Sprintf("%-20s %s:", m.FuncName, m.Field)}
	if m.Extra != 0 {
		parts = append(parts, fmt.Sprintf("extra=%s", FormatRegSet(m.Extra)))
	}
	if m.Missing != 0 {
		parts = append(parts, fmt.Sprintf("missing=%s", FormatRegSet(m.Missing)))
	}
	parts = append(parts, fmt.Sprintf("(declared=%s detected=%s)", FormatRegSet(m.Declared), FormatRegSet(m.Detected)))
	return strings.Join(parts, " ")
}

// VerifyABI compares declared ABIs (from .a80 comments) against detected ABIs
// (from register analysis). Returns mismatches.
// The funcMap maps function names to their entry addresses in the binary.
func VerifyABI(declared []*DeclaredFunc, detected map[uint16]*FuncRegInfo, funcMap map[string]uint16) []*ABIMismatch {
	var mismatches []*ABIMismatch

	for _, df := range declared {
		addr, ok := funcMap[df.Name]
		if !ok {
			continue // function not found in binary — skip
		}
		det, ok := detected[addr]
		if !ok {
			continue // no register analysis for this function
		}

		// Compare IN (strip F from both sides — F as input is noisy)
		declIn := df.In &^ RegF
		detIn := det.In &^ RegF
		if declIn != detIn {
			extra := detIn &^ declIn
			missing := declIn &^ detIn
			if extra != 0 || missing != 0 {
				mismatches = append(mismatches, &ABIMismatch{
					FuncName: df.Name,
					Addr:     addr,
					Field:    "IN",
					Declared: declIn,
					Detected: detIn,
					Extra:    extra,
					Missing:  missing,
				})
			}
		}

		// Compare OUT
		declOut := df.Out
		detOut := det.Out
		if declOut != detOut {
			extra := detOut &^ declOut
			missing := declOut &^ detOut
			if extra != 0 || missing != 0 {
				mismatches = append(mismatches, &ABIMismatch{
					FuncName: df.Name,
					Addr:     addr,
					Field:    "OUT",
					Declared: declOut,
					Detected: detOut,
					Extra:    extra,
					Missing:  missing,
				})
			}
		}

		// Compare CLOBBER (strip F — almost everything clobbers F)
		declClob := df.Clobber &^ RegF
		detClob := det.Clobber &^ RegF
		if declClob != detClob {
			extra := detClob &^ declClob
			missing := declClob &^ detClob
			if extra != 0 || missing != 0 {
				mismatches = append(mismatches, &ABIMismatch{
					FuncName: df.Name,
					Addr:     addr,
					Field:    "CLOBBER",
					Declared: declClob,
					Detected: detClob,
					Extra:    extra,
					Missing:  missing,
				})
			}
		}
	}

	return mismatches
}
