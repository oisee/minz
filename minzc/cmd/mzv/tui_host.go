package main

// TUI host functions for MZV — universal terminal rendering.
//
// Implements the tui_* @extern functions declared in stdlib/tui/render.nanz.
// Renders to stdout using ANSI escape sequences.
//
// Display mode is auto-detected: if any tui_* function is called, MZV enters
// TUI mode and suppresses the ZX Spectrum frame renderer at exit. Programs
// use either ZX mode (zx_poke/zx_halt) or TUI mode (tui_goto/tui_putch),
// never both.

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/minz/minzc/pkg/mir2"
)

// Box-drawing code points (matched to render.nanz BOX_* constants).
var boxChars = map[byte]string{
	1: "┌", // BOX_TL
	2: "┐", // BOX_TR
	3: "└", // BOX_BL
	4: "┘", // BOX_BR
	5: "─", // BOX_H
	6: "│", // BOX_V
}

// registerTUIHosts installs tui_* host functions on the VM.
// Output goes to stdout (the TUI IS the program output).
// stdinForTUI is set by main.go to share the stdin channel with TUI hosts.
var stdinForTUI <-chan byte

func registerTUIHosts(vm *mir2.VM, headless bool, trace bool) {
	termW := 80
	termH := 24
	out := os.Stdout

	// Helper: read null-terminated string from VM heap.
	readStr := func(ptr int64) string {
		var buf []byte
		for i := int64(0); i < 4096; i++ {
			b := vm.ReadHeap(ptr+i, 1)
			if b == nil || b[0] == 0 {
				break
			}
			buf = append(buf, b[0])
		}
		return string(buf)
	}

	// ── Cursor positioning ──────────────────────────────────────────
	vm.Hosts["tui_goto"] = func(args []mir2.Value) ([]mir2.Value, error) {
x, y := int(args[0].I), int(args[1].I)
		fmt.Fprintf(out, "\033[%d;%dH", y+1, x+1) // ANSI is 1-based
		return nil, nil
	}

	// ── Color ───────────────────────────────────────────────────────
	vm.Hosts["tui_color"] = func(args []mir2.Value) ([]mir2.Value, error) {
fg, bg, bright := int(args[0].I), int(args[1].I), int(args[2].I)
		fgCode := 30 + fg
		bgCode := 40 + bg
		if bright != 0 {
			fgCode += 60
			bgCode += 60
		}
		fmt.Fprintf(out, "\033[%d;%dm", fgCode, bgCode)
		return nil, nil
	}

	vm.Hosts["tui_reset"] = func(_ []mir2.Value) ([]mir2.Value, error) {
fmt.Fprintf(out, "\033[0m")
		return nil, nil
	}

	// ── Screen operations ───────────────────────────────────────────
	vm.Hosts["tui_clear"] = func(_ []mir2.Value) ([]mir2.Value, error) {
fmt.Fprintf(out, "\033[2J\033[H")
		return nil, nil
	}

	vm.Hosts["tui_putch"] = func(args []mir2.Value) ([]mir2.Value, error) {
ch := byte(args[0].I)
		if s, ok := boxChars[ch]; ok {
			fmt.Fprint(out, s)
		} else {
			fmt.Fprintf(out, "%c", ch)
		}
		return nil, nil
	}

	vm.Hosts["tui_puts"] = func(args []mir2.Value) ([]mir2.Value, error) {
if len(args) > 0 {
			s := readStr(args[0].I)
			fmt.Fprint(out, s)
		}
		return nil, nil
	}

	// ── Dimensions ──────────────────────────────────────────────────
	vm.Hosts["tui_width"] = func(_ []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: int64(termW)}}, nil
	}

	vm.Hosts["tui_height"] = func(_ []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: int64(termH)}}, nil
	}

	// ── Input ───────────────────────────────────────────────────────
	vm.Hosts["tui_read_key"] = func(_ []mir2.Value) ([]mir2.Value, error) {
		if headless {
			return []mir2.Value{{I: 0}}, nil // no key
		}

		// Non-blocking read from shared stdin channel.
		if stdinForTUI == nil {
			return []mir2.Value{{I: 0}}, nil
		}
		select {
		case b := <-stdinForTUI:
			// Ctrl+C / Ctrl+D → exit
			if b == 3 || b == 4 {
				return []mir2.Value{{I: 0}}, fmt.Errorf("user exit (ctrl+c/d)")
			}
			return []mir2.Value{{I: int64(b)}}, nil
		default:
			return []mir2.Value{{I: 0}}, nil // no key available
		}
	}

	// Legacy blocking tui_read_key kept for reference but not used.
	_ = func(_ []mir2.Value) ([]mir2.Value, error) {
		buf := make([]byte, 8)
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return []mir2.Value{{I: 147}}, nil // F8 on EOF
		}

		b := buf[0]
		if b == 0x1B && n >= 3 && buf[1] == '[' {
			switch buf[2] {
			case 'A':
				return []mir2.Value{{I: 128}}, nil // KEY_UP
			case 'B':
				return []mir2.Value{{I: 129}}, nil // KEY_DOWN
			case 'C':
				return []mir2.Value{{I: 131}}, nil // KEY_RIGHT
			case 'D':
				return []mir2.Value{{I: 130}}, nil // KEY_LEFT
			}
			if n >= 4 && buf[n-1] == '~' {
				code := string(buf[2 : n-1])
				switch code {
				case "11", "1":
					return []mir2.Value{{I: 140}}, nil // F1
				case "12", "2":
					return []mir2.Value{{I: 141}}, nil // F2
				case "13":
					return []mir2.Value{{I: 142}}, nil // F3
				case "15":
					return []mir2.Value{{I: 144}}, nil // F5
				case "17":
					return []mir2.Value{{I: 145}}, nil // F6
				case "18":
					return []mir2.Value{{I: 146}}, nil // F7
				case "19":
					return []mir2.Value{{I: 147}}, nil // F8
				}
			}
			return []mir2.Value{{I: int64(KEY_ESC)}}, nil
		}

		switch b {
		case 0x1B:
			return []mir2.Value{{I: 27}}, nil
		case '\r', '\n':
			return []mir2.Value{{I: 13}}, nil
		case '\t':
			return []mir2.Value{{I: 9}}, nil
		case 0x7F, 0x08:
			return []mir2.Value{{I: 8}}, nil
		default:
			return []mir2.Value{{I: int64(b)}}, nil
		}
	}

	vm.Hosts["tui_read_line"] = func(args []mir2.Value) ([]mir2.Value, error) {
bufPtr := args[0].I
		maxLen := int(args[1].I)

		if headless {
			reader := bufio.NewReader(os.Stdin)
			line, err := reader.ReadString('\n')
			if err != nil && len(line) == 0 {
				return []mir2.Value{{I: 0}}, nil
			}
			line = strings.TrimRight(line, "\r\n")
			if len(line) > maxLen {
				line = line[:maxLen]
			}
			data := append([]byte(line), 0)
			vm.WriteHeapBytes(bufPtr, data)
			return []mir2.Value{{I: int64(len(line))}}, nil
		}

		fmt.Fprint(out, "\033[?25h") // show cursor
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		fmt.Fprint(out, "\033[?25l") // hide cursor

		if len(line) > maxLen {
			line = line[:maxLen]
		}
		data := append([]byte(line), 0)
		vm.WriteHeapBytes(bufPtr, data)
		return []mir2.Value{{I: int64(len(line))}}, nil
	}

	if trace {
		fmt.Fprintf(os.Stderr, "mzv: TUI host functions registered (%dx%d)\n", termW, termH)
	}
}

const KEY_ESC = 27
