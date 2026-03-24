package main

// Screen host functions for MZV — universal TUI framework.
//
// Architecture: the compiler emits sel_register_str/sel_register_int calls
// to describe screen fields, then sel_show() to collect input. On MZV these
// are host functions that render a TUI selection screen with focus, colors,
// and keyboard navigation. On Z80/CP/M the same calls use BDOS/VT100.

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

		// Determine mode
		shouldRead := stdinIsPipe || !headless

		if shouldRead && !stdinIsPipe {
			// Interactive TUI mode — full screen with focus and colors
			selShowTUI(fields, trace)
		} else if shouldRead && stdinIsPipe {
			// Piped input — read values from stdin
			selShowPiped(fields, trace)
		} else {
			// Headless, no pipe — auto-execute with defaults
			if trace {
				selShowTrace(fields)
			}
		}

		// Write values back to VM heap buffers
		for _, f := range fields {
			if f.ty == 'c' && f.bufPtr != 0 {
				data := append([]byte(f.value), 0)
				vm.WriteHeapBytes(f.bufPtr, data)
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

	// verbose: "mzv: screen host functions registered"
}

// ── TUI rendering for sel_show ──────────────────────────────────────────

const (
	termW = 80
	termH = 24
)

// ANSI helpers — output to stdout (the TUI IS the program output)
func ansiGoto(x, y int)            { fmt.Fprintf(os.Stdout, "\033[%d;%dH", y+1, x+1) }
func ansiColor(fg, bg int)         { fmt.Fprintf(os.Stdout, "\033[%d;%dm", fg, bg) }
func ansiReset()                   { fmt.Fprint(os.Stdout, "\033[0m") }
func ansiClear()                   { fmt.Fprint(os.Stdout, "\033[2J\033[H") }
func ansiShowCursor()              { fmt.Fprint(os.Stdout, "\033[?25h") }
func ansiHideCursor()              { fmt.Fprint(os.Stdout, "\033[?25l") }

func selRenderTUI(fields []*selField, focus int, title string) {
	ansiClear()

	// Title bar: white on blue
	ansiColor(97, 104)
	ansiGoto(0, 0)
	text := "  " + title
	fmt.Fprint(os.Stdout, text)
	for i := len(text); i < termW; i++ {
		fmt.Fprint(os.Stdout, " ")
	}
	ansiReset()

	// Fields
	for i, f := range fields {
		row := 2 + i
		ansiGoto(2, row)

		// Label (cyan)
		ansiColor(36, 40)
		label := f.name
		fmt.Fprintf(os.Stdout, "%-12s", label)

		// Value box
		if i == focus {
			ansiColor(97, 104) // white on blue = focused
		} else {
			ansiColor(37, 40) // white on black = normal
		}
		fmt.Fprint(os.Stdout, "[")
		val := f.value
		width := f.length
		if f.ty == 'i' {
			width = 6
		}
		if width < 1 {
			width = 10
		}
		if len(val) > width {
			val = val[:width]
		}
		fmt.Fprintf(os.Stdout, "%-*s", width, val)
		fmt.Fprint(os.Stdout, "]")
		ansiReset()
	}

	// Buttons row
	btnRow := 2 + len(fields) + 1
	ansiGoto(2, btnRow)
	if focus == len(fields) {
		ansiColor(30, 107) // black on bright white = focused
	} else {
		ansiColor(97, 100) // bright white on dark = normal
	}
	fmt.Fprint(os.Stdout, "[F8=Execute]")
	ansiReset()
	fmt.Fprint(os.Stdout, "  ")
	if focus == len(fields)+1 {
		ansiColor(30, 107)
	} else {
		ansiColor(97, 100)
	}
	fmt.Fprint(os.Stdout, "[F3=Back]")
	ansiReset()

	// Status bar
	ansiColor(37, 44)
	ansiGoto(0, termH-1)
	status := "  TAB=Next  Enter=Edit  F8=Execute  F3=Back"
	fmt.Fprint(os.Stdout, status)
	for i := len(status); i < termW; i++ {
		fmt.Fprint(os.Stdout, " ")
	}
	ansiReset()
}

func selShowTUI(fields []*selField, trace bool) {
	focus := 0
	totalItems := len(fields) + 2 // fields + Execute + Back buttons
	title := "Selection Screen"

	for {
		selRenderTUI(fields, focus, title)

		// Read key
		buf := make([]byte, 8)
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break // execute on EOF
		}

		b := buf[0]

		// ESC sequences
		if b == 0x1B && n >= 3 && buf[1] == '[' {
			// Check for function keys
			if n >= 4 && buf[n-1] == '~' {
				code := string(buf[2 : n-1])
				switch code {
				case "19": // F8
					break
				case "13": // F3
					break
				}
			}
			switch buf[2] {
			case 'A': // Up
				if focus > 0 {
					focus--
				}
			case 'B': // Down
				if focus < totalItems-1 {
					focus++
				}
			}
			if n >= 4 && buf[n-1] == '~' {
				code := string(buf[2 : n-1])
				if code == "19" || code == "13" {
					break // F8 or F3 → execute
				}
			}
			continue
		}

		switch b {
		case '\t': // TAB
			focus = (focus + 1) % totalItems
		case '\r', '\n': // Enter
			if focus == len(fields) { // Execute button
				goto done
			}
			if focus == len(fields)+1 { // Back button
				goto done
			}
			// Edit focused field
			if focus < len(fields) {
				f := fields[focus]
				row := 2 + focus
				ansiGoto(15, row)
				ansiColor(97, 104)
				ansiShowCursor()

				reader := bufio.NewReader(os.Stdin)
				line, _ := reader.ReadString('\n')
				line = strings.TrimRight(line, "\r\n")
				ansiHideCursor()
				ansiReset()
				if line != "" {
					f.value = line
				}
			}
		case 0x1B: // bare ESC
			goto done
		}
	}
done:
	ansiClear()
	ansiGoto(0, 0)
	ansiReset()
}

func selShowPiped(fields []*selField, trace bool) {
	reader := bufio.NewReader(os.Stdin)
	for _, f := range fields {
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			break
		}
		line = strings.TrimRight(line, "\r\n")
		if line != "" {
			f.value = line
		}
	}
}

func selShowTrace(fields []*selField) {
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
