package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Exact production text.print consumer; Go strconv and a bounded writer model
// independently determine bytes/state. Book v7 §4 (concrete impl dispatch),
// §2.6 (caller-owned bytes), §18.1 (CString NUL) + library capacity contract.
func TestOriginalTypedPrintNativeFullState(t *testing.T) {
	testTypedPrintNativeState(t, false)
}
func TestTypedPrintImportedFormatterBoundaryNative(t *testing.T) {
	testTypedPrintNativeState(t, true)
}
func testTypedPrintNativeState(t *testing.T, extended bool) {
	const path = "../../../examples/nanz/typed_print.nanz"
	source, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	dependency, e := os.ReadFile("../../../stdlib/text/print.nanz")
	if e != nil {
		t.Fatal(e)
	}
	type call struct {
		Entry string
		Kind  string
		Value int
		Cap   int
	}
	calls := []call{{"main", "main", 65535, 32}, {"probe_zero", "zero", 0, 4}, {"probe_overflow", "overflow", 1000, 3}, {"main", "main", 65535, 32}, {"probe_overflow", "overflow", 1000, 3}, {"probe_zero", "zero", 0, 4}}
	// Additional address-free workload entries call the original imported formatter,
	// not copied/reduced decimal logic. The original main/probes are untouched.
	drivers := ""
	for _, v := range []int{0, 1, 9, 10, 99, 100, 255, 256, 999, 1000, 32767, 32768, 65535} {
		for _, cap := range []int{0, 1, 2, 3, 4, 5, 6, 32} {
			name := fmt.Sprintf("oracle_decimal_%d_%d", v, cap)
			drivers += fmt.Sprintf("\nfun %s() -> u16 { writer.init(&output,%d)\n dec.value=%d\n dec.write_to(&writer)\n writer.finish_c()\n return writer.len }\n", name, cap, v)
			calls = append(calls, call{name, "decimal", v, cap})
		}
	}
	parse := func(extra bool) *hir.Module {
		text := string(source)
		if extra {
			text += drivers
		}
		h, e := nanz.ParseWithOpts(text, "typed_print.nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	if !extended {
		calls = calls[:6]
	}
	raw := mir2.NewVM(hir.LowerModule(parse(extended)))
	raw.MaxSteps = 1000000
	steps, e := CompileHIRSteps(parse(extended))
	if e != nil {
		t.Fatal(e)
	}
	final := mir2.NewVM(steps.MIR2Module)
	final.MaxSteps = 1000000
	asm := "ORG 0x400\n" + steps.Assembly
	if d := os.Getenv("MINZ_TYPED_PRINT_NATIVE_ARTIFACTS"); d != "" {
		os.WriteFile(filepath.Join(d, "before.a80"), []byte(asm), 0644)
		os.WriteFile(filepath.Join(d, "before.mir2"), []byte(steps.MIR2Module.Dump()), 0644)
	}
	assembled, e := z80asm.NewAssembler().AssembleString(asm)
	if e != nil || len(assembled.Errors) != 0 {
		t.Fatalf("assembly%v %v", e, assembled.Errors)
	}
	platform := emulator.NewRemogattoZ80WithScreen()
	platform.SetROMEnd(0)
	z := platform.RemogattoZ80
	if e = z.LoadMemory(0x400, assembled.Binary); e != nil {
		t.Fatal(e)
	}
	globalSizes := map[string]int{"output": 32, "writer": 7, "text": 2, "dec": 2, "small": 1}
	mutable := make([]bool, len(assembled.Binary))
	for name, n := range globalSizes {
		addr, ok := assembled.Symbols[name]
		if !ok {
			t.Fatalf("global%s", name)
		}
		for j := 0; j < n; j++ {
			mutable[addr-0x400+j] = true
		}
	}
	// Named compiler spill declarations are mutable implementation state, not code.

	for _, line := range assembled.Listing {
		source := strings.TrimSpace(line.SourceLine)
		if strings.HasPrefix(source, "_spill_") && strings.Contains(source, ":") {
			for j := range line.Bytes {
				mutable[line.Address-0x400+j] = true
			}
		}
	}

	expectedOutput := make([]byte, 32)
	expectedDec := 0
	expectedSmall := 0
	expectedText := false
	snapshots := []map[string]any{}
	for i, c := range calls {
		text := strconv.Itoa(c.Value)
		if c.Kind == "main" {
			text = "score=65535 u8=175"
			expectedDec = 65535
			expectedSmall = 175
			expectedText = true
		} else if c.Kind == "zero" {
			expectedSmall = 0
		} else {
			expectedDec = c.Value
		}
		n := len(text)
		overflow := byte(0)
		if n >= c.Cap {
			n = c.Cap
			overflow = 1
		}
		copy(expectedOutput, []byte(text)[:n])
		writes := append([]byte(nil), []byte(text)[:n]...)
		if n < c.Cap {
			expectedOutput[n] = 0
			writes = append(writes, 0)
		}
		result := n
		if c.Kind == "overflow" {
			result = int(overflow)
		}
		for mode, vm := range map[string]*mir2.VM{"raw": raw, "final": final} {
			addr := vm.GlobalAddr("output")
			actualWrites := []byte{}
			actualOffsets := []int64{}
			vm.ObserveDataAccess = func(a mir2.VMDataAccess) {
				if a.Op == mir2.OpStore && a.Addr >= addr && a.Addr < addr+32 {
					actualWrites = append(actualWrites, a.Value)
					actualOffsets = append(actualOffsets, a.Addr-addr)
				}
			}
			r, e := vm.Call(c.Entry, nil)
			if e != nil || len(r) != 1 || r[0].I != int64(result) {
				t.Fatalf("%s call%d%s result%v err%v want%d", mode, i, c.Entry, r, e, result)
			}
			if !bytes.Equal(actualWrites, writes) {
				t.Fatalf("%s call%d orderedoutput%v want%v", mode, i, actualWrites, writes)
			}
			for j, off := range actualOffsets {
				if off != int64(j) {
					t.Fatalf("%s outputorder%d", mode, off)
				}
			}
			verifyTypedPrintState(t, mode, c.Entry, vm.ReadHeap, func(n string) int64 { return vm.GlobalAddr(n) }, expectedOutput, n, c.Cap, overflow, expectedDec, expectedSmall, expectedText)
		}
		mem := z.Memory()
		mem[0xeff0], mem[0xeff1] = 0, 3
		mem[0xe000], mem[0xeff2] = 0x91, 0xa3
		profiler := emulator.NewProfiler()
		z.SetProfiler(profiler)
		z.SetRegisters(emulator.Registers{PC: uint16(assembled.Symbols[c.Entry]), SP: 0xeff0})
		done := false
		for j := 0; j < 1000000; j++ {
			if z.GetPC() == 0x300 && z.GetSP() == 0xeff2 {
				done = true
				break
			}
			if z.GetSP() <= 0xe000 || z.GetSP() > 0xeff2 {
				t.Fatal("stackbounds")
			}
			z.Step()
		}
		got := int(z.GetRegisters().HL)

		if c.Kind == "overflow" {
			got = int(z.GetRegisters().A)
		}
		if !done || got != result || mem[0xe000] != 0x91 || mem[0xeff2] != 0xa3 {
			t.Fatalf("nativecall%d%s returned%t got%d want%d", i, c.Entry, done, got, result)
		}
		verifyTypedPrintState(t, "native", c.Entry, func(a int64, n int) []byte { return mem[a : a+int64(n)] }, func(n string) int64 { return int64(assembled.Symbols[n]) }, expectedOutput, n, c.Cap, overflow, expectedDec, expectedSmall, expectedText)
		for address, count := range profiler.WriteCount {
			if count == 0 {
				continue
			}
			legal := address > 0xe000 && address <= 0xeff1
			if address >= 0x400 && address < 0x400+len(mutable) && mutable[address-0x400] {
				legal = true
			}
			if !legal {
				t.Fatalf("native write outside globals/spills/stack call%d addr%04x", i, address)
			}
		}
		for j, b := range assembled.Binary {
			if !mutable[j] && mem[0x400+j] != b {
				t.Fatalf("immutableimagechangedcall%daddr%04x", i, j+0x400)
			}
		}
		snapshots = append(snapshots, map[string]any{"entry": c.Entry, "expected_output": append([]byte(nil), expectedOutput...), "len": n, "cap": c.Cap, "overflow": overflow, "dec": expectedDec, "small": expectedSmall, "expected_result": result})
	}
	if dir := os.Getenv("MINZ_TYPED_PRINT_NATIVE_ARTIFACTS"); dir != "" {
		mode := "original"
		if extended {
			mode = "formatter-boundaries"
		}
		dir = filepath.Join(dir, mode)
		if e = os.MkdirAll(dir, 0755); e != nil {
			t.Fatal(e)
		}
		b, _ := json.MarshalIndent(snapshots, "", "  ")
		for name, data := range map[string][]byte{"source.nanz": source, "dependency-text-print.nanz": dependency, "driver.nanz": []byte(drivers), "target.a80": []byte(asm), "target.bin": assembled.Binary, "oracle.json": b} {
			if e = os.WriteFile(filepath.Join(dir, name), data, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	t.Logf("exact typed_print productionstdlib:%d original/workload calls; all44globalbytes/relocatedpointers/VMorderedwrites/nativePCSPguardsimage; %d B SHA%x", len(calls), len(assembled.Binary), sha256.Sum256(assembled.Binary))
}

func verifyTypedPrintState(t *testing.T, mode, entry string, read func(int64, int) []byte, addr func(string) int64, output []byte, n, cap int, overflow byte, dec, small int, textSet bool) {
	t.Helper()
	if !bytes.Equal(read(addr("output"), 32), output) {
		t.Fatalf("%s %s fulloutput%q want%q", mode, entry, read(addr("output"), 32), output)
	}
	word := func(x int) []byte { return []byte{byte(x), byte(x >> 8)} }
	writer := append(word(int(addr("output"))), word(n)...)
	writer = append(writer, word(cap)...)
	writer = append(writer, overflow)
	if !bytes.Equal(read(addr("writer"), 7), writer) || !bytes.Equal(read(addr("dec"), 2), word(dec)) || read(addr("small"), 1)[0] != byte(small) {
		t.Fatalf("%s %s writer/dec/smallstate", mode, entry)
	}
	ptr := binary.LittleEndian.Uint16(read(addr("text"), 2))
	if textSet {
		if !bytes.Equal(read(int64(ptr), 5), []byte(" u8=\x00")) {
			t.Fatalf("%s textpointer%04x", mode, ptr)
		}
	} else if ptr != 0 {
		t.Fatalf("%s initialtextpointer", mode)
	}
}
