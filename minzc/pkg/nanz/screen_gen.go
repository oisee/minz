package nanz

// Built-in @screen metafunction: generates ABAP-style selection screen code.
//
// Usage in Nanz:
//   @screen("Material Report") {
//       field "Material" length 10 default "*"
//       field "Plant" length 6
//       int   "Count" default 10
//       button "Execute" key F8
//       button "Back" key F3
//   }
//
// Generates:
//   - @extern tui_* declarations
//   - Enum Action { None, Execute, Back }
//   - Global field buffers (buf_<name>, val_<name>)
//   - screen_init(), screen_render(), screen_handle_key(), screen_show()
//   - Per-field accessor functions: screen_<name>() -> ^u8 / u16

import (
	"fmt"
	"strings"
)

// screenField is a parsed field from the @screen block.
type screenField struct {
	keyword  string // "field", "int", "button"
	name     string // label text (e.g. "Material")
	safeName string // snake_case identifier (e.g. "material")
	length   int    // buffer length (text fields)
	defStr   string // default string value
	defInt   int    // default integer value
	fkey     string // function key name (F3, F5, F8)
	fkeyCode int    // function key code (142, 144, 147)
	action   string // action name for button (Execute, Back)
	index    int    // field index in screen order
	row      int    // display row
}

// generateScreenSource generates Nanz source for an @screen declaration.
func generateScreenSource(title string, block []metaBlockNode) (string, error) {
	// Parse fields from block nodes
	fields, err := parseScreenFields(block)
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	emitExterns(&sb)
	emitConstants(&sb)
	emitGlobals(&sb, title, fields)
	emitHelpers(&sb)
	emitScreenInit(&sb, title, fields)
	emitScreenRender(&sb, title, fields)
	emitScreenHandleKey(&sb, fields)
	emitScreenShow(&sb)
	emitAccessors(&sb, fields)

	return sb.String(), nil
}

func parseScreenFields(block []metaBlockNode) ([]screenField, error) {
	var fields []screenField
	row := 2 // start after title bar (row 0) + blank (row 1)
	btnCount := 0

	for i, n := range block {
		f := screenField{
			keyword: n.keyword,
			index:   i,
		}

		switch n.keyword {
		case "field":
			if len(n.args) < 1 {
				return nil, fmt.Errorf("field requires a label: field \"Name\" ...")
			}
			f.name = n.args[0]
			f.safeName = toSnake(f.name)
			f.length = 10
			if v, ok := n.kwargs["length"]; ok {
				fmt.Sscanf(v, "%d", &f.length)
			}
			if v, ok := n.kwargs["default"]; ok {
				f.defStr = v
			}
			f.row = row
			row++

		case "int":
			if len(n.args) < 1 {
				return nil, fmt.Errorf("int requires a label: int \"Name\" ...")
			}
			f.name = n.args[0]
			f.safeName = toSnake(f.name)
			if v, ok := n.kwargs["default"]; ok {
				fmt.Sscanf(v, "%d", &f.defInt)
			}
			f.row = row
			row++

		case "button":
			if len(n.args) < 1 {
				return nil, fmt.Errorf("button requires a label: button \"Execute\" ...")
			}
			f.name = n.args[0]
			f.safeName = toSnake(f.name)

			// Map button name to action
			switch strings.ToLower(f.name) {
			case "execute", "run", "ok":
				f.action = "Execute"
			case "back", "cancel", "exit":
				f.action = "Back"
			default:
				f.action = "Execute" // default action
			}

			// Function key
			f.fkey = "F8" // default
			if v, ok := n.kwargs["key"]; ok {
				f.fkey = v
			}
			switch f.fkey {
			case "F1":
				f.fkeyCode = 140
			case "F2":
				f.fkeyCode = 141
			case "F3":
				f.fkeyCode = 142
			case "F4":
				f.fkeyCode = 143
			case "F5":
				f.fkeyCode = 144
			case "F8":
				f.fkeyCode = 147
			default:
				f.fkeyCode = 147
			}

			if btnCount == 0 {
				row++ // blank line before buttons
			}
			f.row = row
			btnCount++

		default:
			return nil, fmt.Errorf("unknown screen element: %q", n.keyword)
		}

		fields = append(fields, f)
	}
	return fields, nil
}

func emitExterns(sb *strings.Builder) {
	sb.WriteString(`@extern fun tui_clear() -> void
@extern fun tui_goto(x: u8, y: u8) -> void
@extern fun tui_color(fg: u8, bg: u8, bright: u8) -> void
@extern fun tui_reset() -> void
@extern fun tui_putch(ch: u8) -> void
@extern fun tui_puts(str: ^u8) -> void
@extern fun tui_read_key() -> u8
@extern fun tui_width() -> u8
@extern fun tui_height() -> u8
@extern fun tui_read_line(buf: ^u8, maxlen: u8) -> u8

`)
}

func emitConstants(sb *strings.Builder) {
	sb.WriteString(`const SCREEN_NONE: u8 = 0
const SCREEN_EXECUTE: u8 = 1
const SCREEN_BACK: u8 = 2

`)
}

func emitGlobals(sb *strings.Builder, title string, fields []screenField) {
	nfields := len(fields)
	sb.WriteString("global _scr_title: ^u8\n")
	fmt.Fprintf(sb, "global _scr_nfields: u8\n")
	sb.WriteString("global _scr_focus: u8\n\n")

	for _, f := range fields {
		switch f.keyword {
		case "field":
			bufSize := f.length + 2
			fmt.Fprintf(sb, "global _buf_%s: [u8; %d]\n", f.safeName, bufSize)
		case "int":
			fmt.Fprintf(sb, "global _val_%s: u16\n", f.safeName)
		}
	}
	_ = nfields
	sb.WriteString("\n")
}

func emitHelpers(sb *strings.Builder) {
	sb.WriteString(`fun _scr_puts_padded(str: ^u8, width: u8) -> void {
    var ptr: ^u8 = str
    var i: u8 = 0
    while i < width {
        if ptr^ != 0 {
            tui_putch(ptr^)
            ptr = ptr + 1
        } else {
            tui_putch(32)
        }
        i = i + 1
    }
}

fun _scr_print_u16(n: u16) -> void {
    var started: u8 = 0
    var d: u16 = n / 10000
    if d > 0 {
        tui_putch(48 + d)
        n = n - d * 10000
        started = 1
    }
    d = n / 1000
    if d > 0 {
        tui_putch(48 + d)
        n = n - d * 1000
        started = 1
    } else {
        if started == 1 { tui_putch(48) }
    }
    d = n / 100
    if d > 0 {
        tui_putch(48 + d)
        n = n - d * 100
        started = 1
    } else {
        if started == 1 { tui_putch(48) }
    }
    d = n / 10
    if d > 0 {
        tui_putch(48 + d)
        n = n - d * 10
    } else {
        if started == 1 { tui_putch(48) }
    }
    tui_putch(48 + n)
}

`)
}

func emitScreenInit(sb *strings.Builder, title string, fields []screenField) {
	fmt.Fprintf(sb, "fun screen_init() -> void {\n")
	fmt.Fprintf(sb, "    _scr_title = c\"%s\"\n", title)
	fmt.Fprintf(sb, "    _scr_nfields = %d\n", len(fields))
	sb.WriteString("    _scr_focus = 0\n")

	for _, f := range fields {
		switch f.keyword {
		case "field":
			if f.defStr != "" {
				// Write default value byte by byte
				for i, ch := range f.defStr {
					fmt.Fprintf(sb, "    _buf_%s[%d] = %d\n", f.safeName, i, ch)
				}
				fmt.Fprintf(sb, "    _buf_%s[%d] = 0\n", f.safeName, len(f.defStr))
			} else {
				fmt.Fprintf(sb, "    _buf_%s[0] = 0\n", f.safeName)
			}
		case "int":
			fmt.Fprintf(sb, "    _val_%s = %d\n", f.safeName, f.defInt)
		}
	}
	sb.WriteString("}\n\n")
}

func emitScreenRender(sb *strings.Builder, title string, fields []screenField) {
	sb.WriteString("fun screen_render() -> void {\n")
	sb.WriteString("    tui_clear()\n")

	// Title bar
	sb.WriteString("    tui_color(7, 4, 1)\n")
	sb.WriteString("    tui_goto(0, 0)\n")
	sb.WriteString("    tui_puts(c\"  \")\n")
	sb.WriteString("    tui_puts(_scr_title)\n")
	sb.WriteString("    var _tw: u8 = tui_width()\n")
	sb.WriteString("    var _pad: u8 = 20\n")
	sb.WriteString("    while _pad < _tw {\n")
	sb.WriteString("        tui_putch(32)\n")
	sb.WriteString("        _pad = _pad + 1\n")
	sb.WriteString("    }\n")
	sb.WriteString("    tui_reset()\n")

	// Per-field rendering
	btnStarted := false
	for _, f := range fields {
		fmt.Fprintf(sb, "    var _f%d: u8 = 0\n", f.index)
		fmt.Fprintf(sb, "    if _scr_focus == %d { _f%d = 1 }\n", f.index, f.index)

		switch f.keyword {
		case "field":
			fmt.Fprintf(sb, "    tui_goto(2, %d)\n", f.row)
			sb.WriteString("    tui_color(6, 0, 0)\n")
			fmt.Fprintf(sb, "    _scr_puts_padded(c\"%s\", 12)\n", f.name)
			fmt.Fprintf(sb, "    if _f%d == 1 {\n", f.index)
			sb.WriteString("        tui_color(7, 4, 1)\n")
			sb.WriteString("    } else {\n")
			sb.WriteString("        tui_color(7, 0, 0)\n")
			sb.WriteString("    }\n")
			sb.WriteString("    tui_putch(91)\n")
			fmt.Fprintf(sb, "    _scr_puts_padded(&_buf_%s, %d)\n", f.safeName, f.length)
			sb.WriteString("    tui_putch(93)\n")
			sb.WriteString("    tui_reset()\n")

		case "int":
			fmt.Fprintf(sb, "    tui_goto(2, %d)\n", f.row)
			sb.WriteString("    tui_color(6, 0, 0)\n")
			fmt.Fprintf(sb, "    _scr_puts_padded(c\"%s\", 12)\n", f.name)
			fmt.Fprintf(sb, "    if _f%d == 1 {\n", f.index)
			sb.WriteString("        tui_color(7, 4, 1)\n")
			sb.WriteString("    } else {\n")
			sb.WriteString("        tui_color(7, 0, 0)\n")
			sb.WriteString("    }\n")
			sb.WriteString("    tui_putch(91)\n")
			fmt.Fprintf(sb, "    _scr_print_u16(_val_%s)\n", f.safeName)
			sb.WriteString("    tui_putch(93)\n")
			sb.WriteString("    tui_reset()\n")

		case "button":
			if !btnStarted {
				fmt.Fprintf(sb, "    tui_goto(2, %d)\n", f.row)
				btnStarted = true
			} else {
				sb.WriteString("    tui_puts(c\"  \")\n")
			}
			fmt.Fprintf(sb, "    if _f%d == 1 {\n", f.index)
			sb.WriteString("        tui_color(0, 7, 1)\n")
			sb.WriteString("    } else {\n")
			sb.WriteString("        tui_color(7, 0, 1)\n")
			sb.WriteString("    }\n")
			sb.WriteString("    tui_putch(91)\n")
			fmt.Fprintf(sb, "    tui_puts(c\"%s=%s\")\n", f.fkey, f.name)
			sb.WriteString("    tui_putch(93)\n")
			sb.WriteString("    tui_reset()\n")
		}
	}

	// Status bar
	sb.WriteString("    var _th: u8 = tui_height()\n")
	sb.WriteString("    tui_color(7, 4, 0)\n")
	sb.WriteString("    tui_goto(0, _th - 1)\n")

	// Build status bar text from buttons
	var statusParts []string
	statusParts = append(statusParts, "TAB=Next", "Enter=Edit")
	for _, f := range fields {
		if f.keyword == "button" {
			statusParts = append(statusParts, fmt.Sprintf("%s=%s", f.fkey, f.name))
		}
	}
	statusText := "  " + strings.Join(statusParts, "  ")
	fmt.Fprintf(sb, "    tui_puts(c\"%s\")\n", statusText)
	fmt.Fprintf(sb, "    _pad = %d\n", len(statusText))
	sb.WriteString("    while _pad < _tw {\n")
	sb.WriteString("        tui_putch(32)\n")
	sb.WriteString("        _pad = _pad + 1\n")
	sb.WriteString("    }\n")
	sb.WriteString("    tui_reset()\n")
	sb.WriteString("}\n\n")
}

func emitScreenHandleKey(sb *strings.Builder, fields []screenField) {
	sb.WriteString("fun screen_handle_key(key: u8) -> u8 {\n")

	// Function key → action mapping
	for _, f := range fields {
		if f.keyword == "button" {
			fmt.Fprintf(sb, "    if key == %d { return SCREEN_%s }\n", f.fkeyCode, strings.ToUpper(f.action))
		}
	}
	sb.WriteString("    if key == 27 { return SCREEN_BACK }\n") // ESC

	// TAB → cycle focus
	fmt.Fprintf(sb, "    if key == 9 {\n")
	sb.WriteString("        _scr_focus = _scr_focus + 1\n")
	fmt.Fprintf(sb, "        if _scr_focus >= %d { _scr_focus = 0 }\n", len(fields))
	sb.WriteString("        return SCREEN_NONE\n")
	sb.WriteString("    }\n")

	// ENTER → activate
	sb.WriteString("    if key == 13 {\n")
	for _, f := range fields {
		switch f.keyword {
		case "button":
			fmt.Fprintf(sb, "        if _scr_focus == %d { return SCREEN_%s }\n", f.index, strings.ToUpper(f.action))
		case "field":
			fmt.Fprintf(sb, "        if _scr_focus == %d {\n", f.index)
			fmt.Fprintf(sb, "            tui_goto(15, %d)\n", f.row)
			sb.WriteString("            tui_color(7, 4, 1)\n")
			fmt.Fprintf(sb, "            var _len: u8 = tui_read_line(&_buf_%s, %d)\n", f.safeName, f.length)
			sb.WriteString("            tui_reset()\n")
			sb.WriteString("        }\n")
		}
	}
	sb.WriteString("        return SCREEN_NONE\n")
	sb.WriteString("    }\n")

	sb.WriteString("    return SCREEN_NONE\n")
	sb.WriteString("}\n\n")
}

func emitScreenShow(sb *strings.Builder) {
	sb.WriteString(`fun screen_show() -> u8 {
    var action: u8 = SCREEN_NONE
    while action == SCREEN_NONE {
        screen_render()
        var key: u8 = tui_read_key()
        action = screen_handle_key(key)
    }
    tui_clear()
    tui_goto(0, 0)
    tui_reset()
    return action
}

`)
}

func emitAccessors(sb *strings.Builder, fields []screenField) {
	for _, f := range fields {
		switch f.keyword {
		case "field":
			fmt.Fprintf(sb, "fun screen_%s() -> ^u8 {\n", f.safeName)
			fmt.Fprintf(sb, "    return &_buf_%s\n", f.safeName)
			sb.WriteString("}\n\n")
		case "int":
			fmt.Fprintf(sb, "fun screen_%s() -> u16 {\n", f.safeName)
			fmt.Fprintf(sb, "    return _val_%s\n", f.safeName)
			sb.WriteString("}\n\n")
		}
	}
}

// toSnake converts "Material Number" → "material_number"
func toSnake(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	// Remove non-alphanumeric except underscore
	var result strings.Builder
	for _, ch := range s {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' {
			result.WriteRune(ch)
		}
	}
	return result.String()
}
