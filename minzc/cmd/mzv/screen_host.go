package main

// Screen host functions for MZV — universal TUI framework.
//
// Architecture: the compiler emits sel_register_str/sel_register_int calls
// to describe screen fields, then sel_show() to collect input. On MZV these
// are host functions that read from stdin and write values to VM heap buffers.
// On Z80/CP/M the same calls are no-ops and the fallback BDOS path runs.

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/mir2"
)

// selField describes one screen field (PARAMETER or screen_add_field).
type selField struct {
	name   string // field label (uppercase)
	ty     byte   // 'c' = text, 'i' = integer
	length int    // display width (text fields)
	value  string // current value (user input or default)
	defInt int64  // default integer value
	bufPtr int64  // VM heap address of text buffer (string fields)
}

// registerScreenHosts registers the universal screen/TUI host functions.
// These override the Z80 no-op stubs so MZV can do interactive input.
func registerScreenHosts(vm *mir2.VM, headless bool, trace bool) {
	var fields []*selField

	// Detect whether stdin is a pipe (has data) or a terminal.
	stdinIsPipe := false
	if fi, err := os.Stdin.Stat(); err == nil {
		stdinIsPipe = fi.Mode()&os.ModeCharDevice == 0
	}

	// Helper: read null-terminated string from VM heap.
	readStr := func(ptr int64) string {
		var buf []byte
		for i := int64(0); i < 256; i++ {
			b := vm.ReadHeap(ptr+i, 1)
			if b == nil || b[0] == 0 {
				break
			}
			buf = append(buf, b[0])
		}
		return string(buf)
	}

	// ── sel_register_str(name_ptr, length, default_ptr, buf_ptr) ─────
	vm.Hosts["sel_register_str"] = func(args []mir2.Value) ([]mir2.Value, error) {
		name := readStr(args[0].I)
		length := int(args[1].I)
		defStr := readStr(args[2].I)
		bufPtr := args[3].I
		fields = append(fields, &selField{
			name: name, ty: 'c', length: length,
			value: defStr, bufPtr: bufPtr,
		})
		if trace {
			fmt.Fprintf(os.Stderr, "  sel_register_str(%q, %d, %q, @%d)\n", name, length, defStr, bufPtr)
		}
		return nil, nil
	}

	// ── sel_register_int(name_ptr, default_val) ─────────────────────
	vm.Hosts["sel_register_int"] = func(args []mir2.Value) ([]mir2.Value, error) {
		name := readStr(args[0].I)
		defInt := args[1].I
		fields = append(fields, &selField{
			name: name, ty: 'i',
			value: fmt.Sprintf("%d", defInt), defInt: defInt,
		})
		if trace {
			fmt.Fprintf(os.Stderr, "  sel_register_int(%q, %d)\n", name, defInt)
		}
		return nil, nil
	}

	// ── sel_show() → u8 (1 = host handled) ──────────────────────────
	vm.Hosts["sel_show"] = func(_ []mir2.Value) ([]mir2.Value, error) {
		if len(fields) == 0 {
			return []mir2.Value{{I: 1}}, nil
		}

		// Determine if we should read input:
		//   - stdin is a pipe → always read (piped test data)
		//   - not headless → interactive, prompt on stderr and read
		//   - headless + no pipe → auto-execute with defaults
		shouldRead := stdinIsPipe || !headless

		if shouldRead {
			// Render selection screen header to stderr
			maxLabel := 0
			for _, f := range fields {
				if len(f.name) > maxLabel {
					maxLabel = len(f.name)
				}
			}

			if !stdinIsPipe {
				// Interactive mode: show a nice box
				fmt.Fprintf(os.Stderr, "\n┌─ Selection Screen ──────────────────┐\n")
				fmt.Fprintf(os.Stderr, "│                                    │\n")
				for _, f := range fields {
					val := f.value
					if val == "" {
						val = strings.Repeat("_", f.length)
					}
					fmt.Fprintf(os.Stderr, "│  %-10s [%-20s]  │\n", f.name, val)
				}
				fmt.Fprintf(os.Stderr, "│                                    │\n")
				fmt.Fprintf(os.Stderr, "│  [Enter=Execute]                   │\n")
				fmt.Fprintf(os.Stderr, "└────────────────────────────────────┘\n\n")
			}

			// Read input for each field
			reader := bufio.NewReader(os.Stdin)
			for _, f := range fields {
				if !stdinIsPipe {
					fmt.Fprintf(os.Stderr, "%s [%s]: ", f.name, f.value)
				}
				line, err := reader.ReadString('\n')
				if err != nil && len(line) == 0 {
					break // EOF with no data → keep defaults
				}
				line = strings.TrimRight(line, "\r\n")
				if line != "" {
					f.value = line
				}
			}
		} else if trace {
			// Headless with no pipe: show what defaults will be used
			fmt.Fprintf(os.Stderr, "\n┌─ Selection Screen ──────────────────┐\n")
			fmt.Fprintf(os.Stderr, "│                                    │\n")
			for _, f := range fields {
				val := f.value
				if val == "" {
					val = strings.Repeat("_", f.length)
				}
				fmt.Fprintf(os.Stderr, "│  %-10s [%-20s]  │\n", f.name, val)
			}
			fmt.Fprintf(os.Stderr, "│                                    │\n")
			fmt.Fprintf(os.Stderr, "│  [Enter=Execute]                   │\n")
			fmt.Fprintf(os.Stderr, "└────────────────────────────────────┘\n\n")
			fmt.Fprintf(os.Stderr, "  sel_show() → auto-execute with defaults\n")
		}

		// Write values back to VM heap buffers
		for _, f := range fields {
			if f.ty == 'c' && f.bufPtr != 0 {
				// Write null-terminated string to the text buffer
				data := append([]byte(f.value), 0)
				vm.WriteHeap(f.bufPtr, data)
			}
			if f.ty == 'i' {
				if v, err := strconv.ParseInt(f.value, 10, 64); err == nil {
					f.defInt = v
				}
			}
		}

		if trace {
			fmt.Fprintf(os.Stderr, "  sel_show() → 1 (host handled, %d fields)\n", len(fields))
		}
		return []mir2.Value{{I: 1}}, nil
	}

	// ── sel_get_int(idx) → u16 ──────────────────────────────────────
	vm.Hosts["sel_get_int"] = func(args []mir2.Value) ([]mir2.Value, error) {
		idx := int(args[0].I)
		if idx >= 0 && idx < len(fields) {
			return []mir2.Value{{I: fields[idx].defInt}}, nil
		}
		return []mir2.Value{{I: 0}}, nil
	}

	// ── sel_register (3-arg backward compat) ────────────────────────
	vm.Hosts["sel_register"] = func(args []mir2.Value) ([]mir2.Value, error) {
		name := readStr(args[0].I)
		ty := byte(args[1].I)
		length := int(args[2].I)
		fields = append(fields, &selField{name: name, ty: ty, length: length})
		if trace {
			fmt.Fprintf(os.Stderr, "  sel_register(%q, '%c', %d)\n", name, ty, length)
		}
		return nil, nil
	}

	fmt.Fprintf(os.Stderr, "mzv: screen host functions registered\n")
}

// registerScreenHostsWithSY wires screen hosts to the SY-UCOMM variable.
func registerScreenHostsWithSY(vm *mir2.VM, headless bool, trace bool, syUcomm *int64) {
	registerScreenHosts(vm, headless, trace)
	// Patch sel_show to set SY-UCOMM on execute
	origShow := vm.Hosts["sel_show"]
	vm.Hosts["sel_show"] = func(args []mir2.Value) ([]mir2.Value, error) {
		result, err := origShow(args)
		*syUcomm = 0x4F4E // "ON" for ONLI
		return result, err
	}
}
