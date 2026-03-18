package nanz

// Compile-time metafunction execution for Nanz.
//
// Metafunctions are declared as `fun @name(...)` and called as `@name(...) { block }`.
// They run on the MIR2 VM at compile time and emit Nanz source text that gets
// parsed and spliced into the calling module.
//
// Architecture:
//   1. Parser sees `fun @name(...)` → stores as metaFuncSrc (not in m.Funcs)
//   2. Parser sees `@name("title") { block }` → triggers metafunction execution
//   3. Go compiles metafun Nanz → HIR → MIR2 → VM
//   4. Go serializes the block as structured data on VM heap
//   5. VM executes metafun, which calls emit() to produce Nanz source
//   6. Go parses emitted Nanz → HIR nodes → spliced into caller module
//
// Host functions available to metafunctions:
//   emit(str_ptr)                — append line to output buffer
//   block_len(block_ptr)         — number of nodes in block
//   node_keyword(block, i)       — keyword of i-th node (e.g. "field", "button")
//   node_arg_count(block, i)     — number of arguments on i-th node
//   node_arg_str(block, i, j)    — j-th string argument of i-th node
//   node_arg_int(block, i, j)    — j-th integer argument of i-th node
//   str_concat(a, b)             — concatenate two strings
//   str_from_int(n)              — integer to decimal string
//   str_eq(a, b)                 — string equality (returns 0 or 1)

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// metaFuncDef stores a metafunction's source for deferred compilation.
type metaFuncDef struct {
	name   string // without @ prefix
	source string // full Nanz source of the function
	line   int
}

// metaBlockNode is one parsed node from a `@name(...) { ... }` block.
// Example: `field "Material" length 18 default "*"`
// → keyword="field", args=["Material"], kwargs={"length": "18", "default": "*"}
type metaBlockNode struct {
	keyword string
	args    []string          // positional string arguments
	kwargs  map[string]string // keyword arguments (name → value)
}

// metaRuntime manages compile-time execution of metafunctions.
type metaRuntime struct {
	hirMod  *hir.Module
	emitted strings.Builder
	vm      *mir2.VM
}

func newMetaRuntime(hirMod *hir.Module) *metaRuntime {
	return &metaRuntime{hirMod: hirMod}
}

// registerHosts installs all metafunction host functions on the VM.
func (mr *metaRuntime) registerHosts(vm *mir2.VM, block []metaBlockNode) {
	mr.vm = vm

	// ── Emit ──────────────────────────────────────────────────────
	vm.Hosts["emit"] = func(args []mir2.Value) ([]mir2.Value, error) {
		s := mr.readCString(args[0].I)
		mr.emitted.WriteString(s)
		mr.emitted.WriteByte('\n')
		return nil, nil
	}

	// ── Block introspection ───────────────────────────────────────
	vm.Hosts["block_len"] = func(_ []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: int64(len(block))}}, nil
	}

	vm.Hosts["node_keyword"] = func(args []mir2.Value) ([]mir2.Value, error) {
		i := int(args[0].I)
		if i < 0 || i >= len(block) {
			return []mir2.Value{mr.allocString("")}, nil
		}
		return []mir2.Value{mr.allocString(block[i].keyword)}, nil
	}

	vm.Hosts["node_arg_count"] = func(args []mir2.Value) ([]mir2.Value, error) {
		i := int(args[0].I)
		if i < 0 || i >= len(block) {
			return []mir2.Value{{I: 0}}, nil
		}
		return []mir2.Value{{I: int64(len(block[i].args))}}, nil
	}

	vm.Hosts["node_arg_str"] = func(args []mir2.Value) ([]mir2.Value, error) {
		i := int(args[0].I)
		j := int(args[1].I)
		if i < 0 || i >= len(block) {
			return []mir2.Value{mr.allocString("")}, nil
		}
		// Check positional args first
		if j >= 0 && j < len(block[i].args) {
			return []mir2.Value{mr.allocString(block[i].args[j])}, nil
		}
		return []mir2.Value{mr.allocString("")}, nil
	}

	// node_kwarg(node_idx, key_ptr) → value string
	vm.Hosts["node_kwarg"] = func(args []mir2.Value) ([]mir2.Value, error) {
		i := int(args[0].I)
		key := mr.readCString(args[1].I)
		if i < 0 || i >= len(block) {
			return []mir2.Value{mr.allocString("")}, nil
		}
		val, ok := block[i].kwargs[key]
		if !ok {
			return []mir2.Value{mr.allocString("")}, nil
		}
		return []mir2.Value{mr.allocString(val)}, nil
	}

	// node_has_kwarg(node_idx, key_ptr) → 0 or 1
	vm.Hosts["node_has_kwarg"] = func(args []mir2.Value) ([]mir2.Value, error) {
		i := int(args[0].I)
		key := mr.readCString(args[1].I)
		if i < 0 || i >= len(block) {
			return []mir2.Value{{I: 0}}, nil
		}
		if _, ok := block[i].kwargs[key]; ok {
			return []mir2.Value{{I: 1}}, nil
		}
		return []mir2.Value{{I: 0}}, nil
	}

	// ── High-level emit helpers ───────────────────────────────────

	// emit_call(fn_name, arg1, arg2, ...) — emit "    fn_name(arg1, arg2, ...)"
	// All args are string pointers. Strings get c"..." wrapped, integers stay bare.
	vm.Hosts["emit_call"] = func(args []mir2.Value) ([]mir2.Value, error) {
		fn := mr.readCString(args[0].I)
		var sb strings.Builder
		sb.WriteString("    ")
		sb.WriteString(fn)
		sb.WriteByte('(')
		for i := 1; i < len(args); i++ {
			if i > 1 {
				sb.WriteString(", ")
			}
			sb.WriteString(mr.readCString(args[i].I))
		}
		sb.WriteByte(')')
		mr.emitted.WriteString(sb.String())
		mr.emitted.WriteByte('\n')
		return nil, nil
	}

	// emit_tui_puts(str) — emit '    tui_puts(c"str")'
	vm.Hosts["emit_tui_puts"] = func(args []mir2.Value) ([]mir2.Value, error) {
		s := mr.readCString(args[0].I)
		fmt.Fprintf(&mr.emitted, "    tui_puts(c\"%s\")\n", s)
		return nil, nil
	}

	// emit_tui_goto(x, y) — emit '    tui_goto(x, y)'
	vm.Hosts["emit_tui_goto"] = func(args []mir2.Value) ([]mir2.Value, error) {
		x, y := args[0].I, args[1].I
		fmt.Fprintf(&mr.emitted, "    tui_goto(%d, %d)\n", x, y)
		return nil, nil
	}

	// emit_tui_color(fg, bg, bright) — emit '    tui_color(fg, bg, bright)'
	vm.Hosts["emit_tui_color"] = func(args []mir2.Value) ([]mir2.Value, error) {
		fg, bg, br := args[0].I, args[1].I, args[2].I
		fmt.Fprintf(&mr.emitted, "    tui_color(%d, %d, %d)\n", fg, bg, br)
		return nil, nil
	}

	// ── String helpers ────────────────────────────────────────────
	vm.Hosts["str_concat"] = func(args []mir2.Value) ([]mir2.Value, error) {
		a := mr.readCString(args[0].I)
		b := mr.readCString(args[1].I)
		return []mir2.Value{mr.allocString(a + b)}, nil
	}

	vm.Hosts["str_from_int"] = func(args []mir2.Value) ([]mir2.Value, error) {
		s := fmt.Sprintf("%d", args[0].I)
		return []mir2.Value{mr.allocString(s)}, nil
	}

	// str_chr(code) → single-character string from ASCII code
	vm.Hosts["str_chr"] = func(args []mir2.Value) ([]mir2.Value, error) {
		ch := byte(args[0].I)
		return []mir2.Value{mr.allocString(string([]byte{ch}))}, nil
	}

	vm.Hosts["str_eq"] = func(args []mir2.Value) ([]mir2.Value, error) {
		a := mr.readCString(args[0].I)
		b := mr.readCString(args[1].I)
		if a == b {
			return []mir2.Value{{I: 1}}, nil
		}
		return []mir2.Value{{I: 0}}, nil
	}
}

// allocString writes a NUL-terminated string to the VM heap.
func (mr *metaRuntime) allocString(s string) mir2.Value {
	data := append([]byte(s), 0)
	return mr.vm.AllocHeap(data)
}

// readCString reads a NUL-terminated string from VM heap.
func (mr *metaRuntime) readCString(addr int64) string {
	var sb strings.Builder
	for off := int64(0); ; off++ {
		data := mr.vm.ReadHeap(addr+off, 1)
		if data == nil || data[0] == 0 {
			break
		}
		sb.WriteByte(data[0])
	}
	return sb.String()
}

// executeMetaFunc compiles and runs a metafunction, returning emitted Nanz source.
func executeMetaFunc(
	metaSrc string,
	funcName string,
	scalarArgs []mir2.Value,
	block []metaBlockNode,
	callerMod *hir.Module,
) (string, error) {
	// 1. Parse metafunction source → HIR
	metaHIR, err := Parse(metaSrc, "meta_"+funcName+".nanz")
	if err != nil {
		return "", fmt.Errorf("metafunc @%s: parse error: %w", funcName, err)
	}

	// 2. HIR → MIR2
	mirMod := hir.LowerModule(metaHIR)

	// Optimize
	for _, f := range mirMod.Funcs {
		mir2.EliminateDeadBlocks(f)
		for {
			p := mir2.PropagateConstants(f)
			c := mir2.FoldConstants(f)
			if !p && !c {
				break
			}
		}
	}

	// 3. Create VM
	vm := mir2.NewVM(mirMod)
	vm.MaxSteps = 1_000_000
	vm.MaxMemory = 1 << 20

	// 4. Register host functions
	mr := newMetaRuntime(callerMod)
	mr.registerHosts(vm, block)

	// 5. Call metafunction
	_, err = vm.Call(funcName, scalarArgs)
	if err != nil {
		return "", fmt.Errorf("metafunc @%s: VM error: %w", funcName, err)
	}

	// 6. Return emitted text
	return mr.emitted.String(), nil
}

// parseMetaBlock parses a { keyword ... , keyword ... } block into structured data.
// Each line is: keyword "string_arg" key value key value, ...
//
// Example:
//
//	{
//	    field "Material" length 18 default "*"
//	    int "Count" default 10
//	    button "Execute" key F8
//	}
func parseMetaBlock(l *lexer) ([]metaBlockNode, error) {
	if _, err := l.eat(tokLBrace); err != nil {
		return nil, err
	}

	var nodes []metaBlockNode

	for !l.is(tokRBrace) && !l.is(tokEOF) {
		// Skip commas/newlines between nodes
		if l.is(tokComma) {
			l.next()
			continue
		}

		// Keyword
		kwTok, err := l.eat(tokIdent)
		if err != nil {
			return nil, fmt.Errorf("line %d: expected keyword in meta block: %w", l.peek().line, err)
		}

		node := metaBlockNode{
			keyword: kwTok.val,
			kwargs:  make(map[string]string),
		}

		// Arguments: string/int literals and keyword-value pairs.
		// Stop when we see a new node keyword on a different line.
		kwLine := kwTok.line
		for !l.is(tokRBrace) && !l.is(tokComma) && !l.is(tokEOF) {
			t := l.peek()

			// If this identifier is on a new line and could be a new node keyword,
			// stop — it starts the next node.
			if t.kind == tokIdent && t.line > kwLine {
				break
			}

			// String literal → positional argument
			if t.kind == tokString {
				l.next()
				// Extract string content from prefixed format
				val := t.val
				if idx := strings.IndexByte(val, 0); idx >= 0 {
					val = val[idx+1:]
				}
				node.args = append(node.args, val)
				continue
			}

			// Integer literal → positional argument or kwarg value
			if t.kind == tokInt {
				l.next()
				node.args = append(node.args, t.val)
				continue
			}

			// Identifier → keyword argument (key value pair on same line)
			if t.kind == tokIdent {
				keyName := t.val
				l.next()
				nt := l.peek()
				if nt.line == t.line && (nt.kind == tokString || nt.kind == tokInt || nt.kind == tokIdent) {
					l.next()
					val := nt.val
					if nt.kind == tokString {
						if idx := strings.IndexByte(val, 0); idx >= 0 {
							val = val[idx+1:]
						}
					}
					node.kwargs[keyName] = val
				} else {
					node.args = append(node.args, keyName)
				}
				continue
			}

			break
		}

		nodes = append(nodes, node)
	}

	if _, err := l.eat(tokRBrace); err != nil {
		return nil, fmt.Errorf("expected closing '}' in meta block: %w", err)
	}

	return nodes, nil
}
