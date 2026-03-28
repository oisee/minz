package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/minz/minzc/pkg/mir2"
	vir "github.com/minz/minzc/pkg/vir"
)

// registerFileHosts adds host filesystem access for self-hosting toolchain.
// Programs can read source files directly from the host filesystem via MZV.
//
// Host functions:
//   @file_size(path: ^u8) -> u16          — returns file size (0 if not found)
//   @file_read(path: ^u8, buf: ^u8) -> u16 — reads entire file into buf, returns bytes read
//   @file_exists(path: ^u8) -> u8         — returns 1 if file exists, 0 otherwise
func registerFileHosts(vm *mir2.VM, baseDir string, trace bool) {
	// Helper: read null-terminated string from VM heap.
	readStr := func(ptr int64) string {
		var buf []byte
		for i := int64(0); i < 4096; i++ {
			b := vm.ReadHeap(ptr+i, 1)
			if b == nil || len(b) == 0 || b[0] == 0 {
				break
			}
			buf = append(buf, b[0])
		}
		return string(buf)
	}

	// Resolve path relative to baseDir.
	resolve := func(path string) string {
		if filepath.IsAbs(path) {
			return path
		}
		return filepath.Join(baseDir, path)
	}

	// @file_size(path: ^u8) -> u16
	vm.Hosts["@file_size"] = func(args []mir2.Value) ([]mir2.Value, error) {
		path := resolve(readStr(args[0].I))
		info, err := os.Stat(path)
		if err != nil {
			if trace {
				fmt.Fprintf(os.Stderr, "  file_size(%q): %v\n", path, err)
			}
			return []mir2.Value{{I: 0}}, nil
		}
		size := info.Size()
		if size > 65535 {
			size = 65535
		}
		if trace {
			fmt.Fprintf(os.Stderr, "  file_size(%q) → %d\n", path, size)
		}
		return []mir2.Value{{I: size}}, nil
	}

	// file_read / @file_read (path: ^u8, buf: ^u8) -> u16
	fileReadFn := func(args []mir2.Value) ([]mir2.Value, error) {
		path := resolve(readStr(args[0].I))
		data, err := os.ReadFile(path)
		if err != nil {
			if trace {
				fmt.Fprintf(os.Stderr, "  file_read(%q): %v\n", path, err)
			}
			return []mir2.Value{{I: 0}}, nil
		}
		if len(data) > 65535 {
			data = data[:65535]
		}
		vm.EnsureHeap(args[1].I + int64(len(data)) + 1)
		vm.WriteHeapBytes(args[1].I, data)
		// Null-terminate
		vm.WriteHeapBytes(args[1].I+int64(len(data)), []byte{0})
		if trace {
			fmt.Fprintf(os.Stderr, "  file_read(%q) → %d bytes at heap@%d\n", path, len(data), args[1].I)
		}
		return []mir2.Value{{I: int64(len(data))}}, nil
	}
	vm.Hosts["@file_read"] = fileReadFn
	vm.Hosts["file_read"] = fileReadFn

	// peek(addr: u16) -> u8 — read byte from VM heap
	// Auto-extends heap if needed (Z80 has 64KB address space).
	vm.Hosts["peek"] = func(args []mir2.Value) ([]mir2.Value, error) {
		addr := args[0].I
		vm.EnsureHeap(addr + 1)
		b := vm.ReadHeap(addr, 1)
		if b == nil || len(b) == 0 {
			return []mir2.Value{{I: 0}}, nil
		}
		return []mir2.Value{{I: int64(b[0])}}, nil
	}

	// poke(addr: u16, val: u8) -> void — write byte to VM heap
	// Auto-extends heap if needed.
	vm.Hosts["poke"] = func(args []mir2.Value) ([]mir2.Value, error) {
		addr := args[0].I
		vm.EnsureHeap(addr + 1)
		vm.WriteHeap(addr, byte(args[1].I))
		return nil, nil
	}

	// @print_u8(val: u8) -> void — print decimal
	vm.Hosts["@print_u8"] = func(args []mir2.Value) ([]mir2.Value, error) {
		fmt.Fprintf(os.Stdout, "%d", args[0].I)
		return nil, nil
	}

	// @print_char(val: u8) -> void — print ASCII character
	vm.Hosts["@print_char"] = func(args []mir2.Value) ([]mir2.Value, error) {
		fmt.Fprintf(os.Stdout, "%c", byte(args[0].I))
		return nil, nil
	}

	// @print_nl() -> void — print newline
	vm.Hosts["@print_nl"] = func(args []mir2.Value) ([]mir2.Value, error) {
		fmt.Fprintln(os.Stdout)
		return nil, nil
	}

	// file_write(path: ^u8, buf: ^u8, len: u16) -> u16
	// Writes len bytes from VM heap to host file. Returns bytes written.
	fileWriteFn := func(args []mir2.Value) ([]mir2.Value, error) {
		path := resolve(readStr(args[0].I))
		length := int(args[2].I)
		vm.EnsureHeap(args[1].I + int64(length))
		data := vm.ReadHeap(args[1].I, length)
		if data == nil {
			return []mir2.Value{{I: 0}}, nil
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			if trace {
				fmt.Fprintf(os.Stderr, "  file_write(%q): %v\n", path, err)
			}
			return []mir2.Value{{I: 0}}, nil
		}
		if trace {
			fmt.Fprintf(os.Stderr, "  file_write(%q) → %d bytes\n", path, length)
		}
		return []mir2.Value{{I: int64(length)}}, nil
	}
	vm.Hosts["@file_write"] = fileWriteFn
	vm.Hosts["file_write"] = fileWriteFn

	// @file_exists(path: ^u8) -> u8
	vm.Hosts["@file_exists"] = func(args []mir2.Value) ([]mir2.Value, error) {
		path := resolve(readStr(args[0].I))
		_, err := os.Stat(path)
		if err != nil {
			return []mir2.Value{{I: 0}}, nil
		}
		return []mir2.Value{{I: 1}}, nil
	}

	// ── Self-hosting regalloc host functions ────────────────────────────

	// @regalloc_lookup(desc_ptr: ^u8) -> u8
	// Reads interference graph + op_bag from VM heap, computes enriched
	// signature, looks up in precomputed tables. Writes assignment at desc_ptr+256.
	// Returns: 1=found, 0=miss.
	//
	// Heap layout at desc_ptr:
	//   [0]    u8   nVregs
	//   [1-2]  u16  nEdges (little-endian)
	//   [3..]  u8[] edges (pairs: v1,v2,v1,v2,... — nEdges*2 bytes)
	//   [+0]   u8[] op_bag (12 bytes: add,sub,mul,cmp,logic,shift,load,store,call,move,const,neg)
	// Output at desc_ptr+256:
	//   u8[nVregs] assignment (physical register index per vreg)
	vm.Hosts["@regalloc_lookup"] = func(args []mir2.Value) ([]mir2.Value, error) {
		ptr := args[0].I
		vm.EnsureHeap(ptr + 512)

		nVregs := int(vm.ReadHeap(ptr, 1)[0])
		edgesLo := vm.ReadHeap(ptr+1, 1)[0]
		edgesHi := vm.ReadHeap(ptr+2, 1)[0]
		nEdges := int(edgesLo) | (int(edgesHi) << 8)

		// Read edges
		var edges [][2]int
		edgeStart := ptr + 3
		for i := 0; i < nEdges; i++ {
			v1 := int(vm.ReadHeap(edgeStart+int64(i*2), 1)[0])
			v2 := int(vm.ReadHeap(edgeStart+int64(i*2+1), 1)[0])
			if v1 > v2 {
				v1, v2 = v2, v1
			}
			edges = append(edges, [2]int{v1, v2})
		}

		// Read op_bag (12 bytes after edges)
		bagStart := edgeStart + int64(nEdges*2)
		bagBytes := vm.ReadHeap(bagStart, 12)
		bag := vir.OpBag{
			Add: int(bagBytes[0]), Sub: int(bagBytes[1]), Mul: int(bagBytes[2]),
			Cmp: int(bagBytes[3]), Logic: int(bagBytes[4]), Shift: int(bagBytes[5]),
			Load: int(bagBytes[6]), Store: int(bagBytes[7]), Call: int(bagBytes[8]),
			Move: int(bagBytes[9]), Const: int(bagBytes[10]), Neg: int(bagBytes[11]),
		}

		// Build shape + compute signature
		shape := vir.InterferenceShape{NVregs: nVregs, Edges: edges}
		sig := vir.EnrichedSignature{
			ShapeHash: shape.Hash(),
			OpBagHash: bag.Hash(),
			NVregs:    nVregs,
		}

		// Lookup in enriched table
		table := vir.GetRegAllocTable()
		enrichedKey := fmt.Sprintf("%d:%d", sig.ShapeHash, sig.OpBagHash)
		entry, ok := table.LookupByKey(enrichedKey)
		if !ok {
			if trace {
				fmt.Fprintf(os.Stderr, "  regalloc_lookup(%dv, %d edges): MISS\n", nVregs, nEdges)
			}
			return []mir2.Value{{I: 0}}, nil
		}

		// Write assignment to desc_ptr+256
		outPtr := ptr + 256
		for i, loc := range entry.Assignment {
			if i >= nVregs {
				break
			}
			vm.WriteHeap(outPtr+int64(i), byte(loc))
		}

		if trace {
			fmt.Fprintf(os.Stderr, "  regalloc_lookup(%dv, %d edges): HIT cost=%d assignment=%v\n",
				nVregs, nEdges, entry.Cost, entry.Assignment)
		}
		return []mir2.Value{{I: 1}}, nil
	}

	// @peephole_match(inst1_ptr: ^u8, inst2_ptr: ^u8, result_ptr: ^u8) -> u8
	// Looks up a 2-instruction pattern in GPU-proven peephole rules.
	// Returns: number of replacement instructions (0=no match).
	// Writes replacement at result_ptr as ':'-separated null-terminated string.
	vm.Hosts["@peephole_match"] = func(args []mir2.Value) ([]mir2.Value, error) {
		inst1 := readStr(args[0].I)
		inst2 := readStr(args[1].I)
		outPtr := args[2].I

		rules := vir.GetPeepholeRules()
		if rules == nil || rules.Size() == 0 {
			return []mir2.Value{{I: 0}}, nil
		}

		rule := rules.Lookup2(inst1, inst2)
		if rule == nil {
			return []mir2.Value{{I: 0}}, nil
		}

		// Write replacement to heap
		repl := []byte(rule.Replacement)
		vm.EnsureHeap(outPtr + int64(len(repl)) + 1)
		vm.WriteHeapBytes(outPtr, repl)
		vm.WriteHeapBytes(outPtr+int64(len(repl)), []byte{0})

		// Count instructions (separated by " : ")
		nInsts := 1
		for i := 0; i < len(repl)-2; i++ {
			if repl[i] == ' ' && repl[i+1] == ':' && repl[i+2] == ' ' {
				nInsts++
			}
		}

		if trace {
			fmt.Fprintf(os.Stderr, "  peephole_match(%q, %q) → %q (%d insts)\n",
				inst1, inst2, rule.Replacement, nInsts)
		}
		return []mir2.Value{{I: int64(nInsts)}}, nil
	}
}
