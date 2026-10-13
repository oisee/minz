package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lizp"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

// Lizp frontend documents byte peek/poke and typed Lanz semantics;
// Book v7 §2.15 memory and §18.1 decimal printing. Token categories follow
// the exact source: comments are ignored, strings have no escape grammar.
func TestOriginalLizpTokenizerWholeMain(t *testing.T) {
	const path = "../../../examples/lizp/self_tokenizer.lizp"
	source, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile("../../../reports/lizp-tokenizer-main/freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	var freeze struct {
		SourceSHA256 string   `json:"source_sha256"`
		MainOutput   string   `json:"main_output"`
		Fixtures     []string `json:"fixtures"`
	}
	if e = json.Unmarshal(data, &freeze); e != nil || freeze.SourceSHA256 != fmt.Sprintf("%x", sha256.Sum256(source)) {
		t.Fatal("freeze", e)
	}
	save := func(name string, v any) {
		dir := os.Getenv("MINZ_LIZP_TOKENIZER_ARTIFACTS")
		if dir == "" {
			return
		}
		p := filepath.Join(dir, name)
		if e = os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			t.Fatal(e)
		}
		var b []byte
		switch x := v.(type) {
		case []byte:
			b = x
		case string:
			b = []byte(x)
		default:
			b, e = json.MarshalIndent(v, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
		}
		if e = os.WriteFile(p, b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	parse := func() *hir.Module {
		h, e := lizp.Compile(string(source), path)
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		raw := hir.LowerModule(parse())
		h := parse()
		s, e := CompileHIRSteps(h, Options{AssertMode: "none", UseGrace: mode != "ordinary", OptSize: mode == "grace-size"})
		if e != nil {
			t.Fatal(mode, e)
		}
		a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
		if e != nil || len(a.Errors) > 0 {
			t.Fatal(e, a.Errors)
		}
		if len(a.Binary) > 0x3000 {
			t.Fatal("image overlaps state")
		}
		save(mode+"/target.a80", s.Assembly)
		save(mode+"/target.bin", a.Binary)
		save(mode+"/raw.mir", raw.Dump())
		save(mode+"/final.mir", s.MIR2Module.Dump())
		save(mode+"/allocation.json", s.Allocation)
		spills := map[int]bool{}
		for _, l := range a.Listing {
			if len(l.Label) > 7 && l.Label[:7] == "_spill_" {
				for j := range l.Bytes {
					spills[l.Address+j] = true
				}
			}
		}
		mainModel := bytes.Repeat([]byte{0xa5}, 65536)
		copy(mainModel[0xc000:], []byte("(+ 1 2)\x00"))
		copy(mainModel[0xbff0:], []byte{7, 0, 7, 0})
		for name, m := range map[string]*mir2.Module{"raw": raw, "final": s.MIR2Module} {
			vm := mir2.NewVM(m)
			vm.EnsureHeap(65536)
			vm.WriteHeapSlice(0, bytes.Repeat([]byte{0xa5}, 65536))
			var out bytes.Buffer
			vm.Hosts["@mir.io.print.u8"] = func(v []mir2.Value) ([]mir2.Value, error) { fmt.Fprintf(&out, "%d", byte(v[0].I)); return nil, nil }
			vm.Hosts["@mir.io.print.nl"] = func(v []mir2.Value) ([]mir2.Value, error) { out.WriteByte('\n'); return nil, nil }
			var events []mir2.VMDataAccess
			vm.ObserveDataAccess = func(a mir2.VMDataAccess) { events = append(events, a) }
			got, e := vm.Call("main", []mir2.Value{{I: 255}})
			if err := lizpTokenizerCheckWrites(events, "main", 0, 7); err != nil {
				t.Fatal(mode, name, err)
			}
			if e != nil || len(got) != 1 || got[0].I != 0 || out.String() != freeze.MainOutput || !bytes.Equal(vm.ReadHeap(0, 65536), mainModel) {
				t.Fatal(mode, name, "wholemain", got, e, out.String(), vm.ReadHeap(0xbff0, 8))
			}
		}
		run := func(z *emulator.RemogattoZ80, entry string, args []int64, expected byte, model []byte) {
			mem := z.Memory()
			mem[0xeff0], mem[0xeff1] = 0, 0x70
			before := append([]byte(nil), mem...)
			r := emulator.Registers{PC: uint16(a.Symbols[entry]), SP: 0xeff0, A: 0x42, BC: 0x5555, DE: 0x6666, HL: 0x7777, IX: 0x8888, IY: 0x9999}
			f := s.MIR2Module.FuncByName(entry)
			for j, v := range args {
				loc := assertParamLocation(f.Contract.Params[j], s.Allocation)
				switch loc.Name {
				case "A":
					r.A = byte(v)
				case "B":
					r.BC = r.BC&255 | uint16(byte(v))<<8
				case "C":
					r.BC = r.BC&0xff00 | uint16(byte(v))
				default:
					t.Fatal("entry ABI", entry, loc)
				}
			}
			z.SetRegisters(r)
			minSP := uint16(0xeff0)
			done := false
			var events []mir2.VMDataAccess
			initialPos := int(mem[0xbff0]) | int(mem[0xbff1])<<8
			for n := 0; n < 1000000; n++ {
				if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
					done = true
					break
				}
				if z.GetSP() < 0xe000 || z.GetSP() > 0xeff2 {
					t.Fatal("stack bounds", entry)
				}
				events = append(events, lizpTokenizerNativeAccess(mem, z.GetPC(), z.GetRegisters())...)
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
			}
			if err := lizpTokenizerCheckWrites(events, entry, initialPos, int(model[0xbff0])|int(model[0xbff1])<<8); err != nil {
				t.Fatal(mode, "native", err)
			}
			if !done || z.GetRegisters().A != expected {
				t.Fatal(mode, "native return", entry, z.GetRegisters(), expected)
			}
			copy(before[0xbff0:0xbff4], model[0xbff0:0xbff4])
			copy(before[0xc000:0xd000], model[0xc000:0xd000])
			copy(before[minSP:0xeff0], mem[minSP:0xeff0])
			for p := range spills {
				before[p] = mem[p]
			}
			if !bytes.Equal(before, mem) {
				for p := range mem {
					if before[p] != mem[p] {
						t.Fatalf("%s %s RAM %x got%x want%x", mode, entry, p, mem[p], before[p])
					}
				}
			}
		}
		z := emulator.NewRemogattoZ80()
		copy(z.Memory(), bytes.Repeat([]byte{0xa5}, 65536))
		z.LoadMemory(0x8000, a.Binary)
		run(z, "main", []int64{255}, 0, mainModel)
		if string(z.GetOutput()) != freeze.MainOutput {
			t.Fatal(mode, "native main output", string(z.GetOutput()))
		}
		for _, input := range freeze.Fixtures {
			model := bytes.Repeat([]byte{0xa5}, 65536)
			copy(model[0xc000:], []byte(input))
			model[0xbff0], model[0xbff1], model[0xbff2], model[0xbff3] = 0, 0, byte(len(input)), byte(len(input)>>8)
			native := emulator.NewRemogattoZ80()
			copy(native.Memory(), model)
			native.LoadMemory(0x8000, a.Binary)
			vms := map[string]*mir2.VM{}
			for name, m := range map[string]*mir2.Module{"raw": raw, "final": s.MIR2Module} {
				vm := mir2.NewVM(m)
				vm.EnsureHeap(65536)
				vm.WriteHeapSlice(0, model)
				vms[name] = vm
			}
			pos := 0
			for n := 0; n <= len(input)+1; n++ {
				tok, next := lizpTokenizerModel([]byte(input), pos)
				pos = next
				model[0xbff0], model[0xbff1] = byte(pos), byte(pos>>8)
				run(native, "next_token", nil, tok, model)
				for name, vm := range vms {
					var events []mir2.VMDataAccess
					vm.ObserveDataAccess = func(a mir2.VMDataAccess) { events = append(events, a) }
					oldPos := int(vm.ReadHeap(0xbff0, 2)[0]) | int(vm.ReadHeap(0xbff0, 2)[1])<<8
					got, e := vm.Call("next_token", nil)
					if err := lizpTokenizerCheckWrites(events, "next_token", oldPos, pos); err != nil {
						t.Fatal(mode, name, input, err)
					}
					if e != nil || len(got) != 1 || got[0].I != int64(tok) || !bytes.Equal(vm.ReadHeap(0, 65536), model) {
						t.Fatal(mode, name, input, n, tok, pos, got, e)
					}
				}
				if tok == 0 {
					break
				}
				if n == len(input)+1 {
					t.Fatal("nontermination", input)
				}
			}
			if len(native.GetOutput()) != 0 {
				t.Fatal("next_token unexpected output")
			}
		}
		save(mode+"/evidence.json", map[string]any{"main_output": freeze.MainOutput, "fixtures": len(freeze.Fixtures), "source_asserts_retained": len(h.Asserts), "source_sha256": freeze.SourceSHA256, "bin_sha256": fmt.Sprintf("%x", sha256.Sum256(a.Binary))})
		t.Log(mode, "original fullmain +", len(freeze.Fixtures), "persistent lexer fixtures raw/final/native PASS")
	}
}
func lizpTokenizerModel(src []byte, pos int) (byte, int) {
	cur := func() byte {
		if pos >= len(src) {
			return 0
		}
		return src[pos]
	}
	ws := func(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\n' }
	for {
		for ws(cur()) {
			pos++
		}
		if cur() != ';' {
			break
		}
		for cur() != 0 && cur() != '\n' {
			pos++
		}
		if cur() != 0 {
			pos++
		}
	}
	c := cur()
	switch {
	case c == 0:
		return 0, pos
	case c == '(':
		return 1, pos + 1
	case c == ')':
		return 2, pos + 1
	case c == '"':
		pos++
		for cur() != 0 && cur() != '"' {
			pos++
		}
		if cur() != 0 {
			pos++
		}
		return 5, pos
	case c >= '0' && c <= '9':
		for cur() >= '0' && cur() <= '9' {
			pos++
		}
		return 4, pos
	default:
		for cur() != 0 && !ws(cur()) && cur() != '(' && cur() != ')' && cur() != '"' && cur() != ';' {
			pos++
		}
		return 3, pos
	}
}

// Poke effects are byte ordered by the source; each advance stores little-endian
// low then high, including unchanged high bytes. Reads must not mutate inputs.
func lizpTokenizerCheckWrites(events []mir2.VMDataAccess, entry string, oldPos, newPos int) error {
	var want []mir2.VMDataAccess
	add := func(addr int, value byte) {
		want = append(want, mir2.VMDataAccess{Op: mir2.OpStore, Addr: int64(addr), Value: value})
	}
	if entry == "main" {
		for j, b := range []byte("(+ 1 2)\x00") {
			add(0xc000+j, b)
		}
		add(0xbff2, 7)
		add(0xbff3, 0)
		add(0xbff0, 0)
		add(0xbff1, 0)
		oldPos = 0
	}
	for pos := oldPos + 1; pos <= newPos; pos++ {
		add(0xbff0, byte(pos))
		add(0xbff1, byte(pos>>8))
	}
	lastSource := int64(0xc000 + oldPos)
	for _, a := range events {
		if a.Op == mir2.OpLoad && a.Addr >= 0xc000 && a.Addr < 0xd000 {
			if a.Addr < lastSource || a.Addr > int64(0xc000+newPos) {
				return fmt.Errorf("%s source read out of cursor order: %v", entry, a)
			}
			lastSource = a.Addr
		}
	}
	var got []mir2.VMDataAccess
	for _, a := range events {
		if a.Op == mir2.OpStore && a.Addr >= 0xbff0 && a.Addr < 0xd000 {
			got = append(got, a)
		}
	}
	if len(got) != len(want) {
		return fmt.Errorf("%s ordered writes got%v want%v", entry, got, want)
	}
	for j := range want {
		if got[j].Addr != want[j].Addr || got[j].Value != want[j].Value {
			return fmt.Errorf("%s write%d got%v want%v", entry, j, got[j], want[j])
		}
	}
	return nil
}
func lizpTokenizerNativeAccess(mem []byte, pc uint16, r emulator.Registers) []mir2.VMDataAccess {
	switch mem[pc] {
	case 0x32:
		addr := uint16(mem[pc+1]) | uint16(mem[pc+2])<<8
		return []mir2.VMDataAccess{{Op: mir2.OpStore, Addr: int64(addr), Value: r.A}}
	case 0x3a:
		addr := uint16(mem[pc+1]) | uint16(mem[pc+2])<<8
		return []mir2.VMDataAccess{{Op: mir2.OpLoad, Addr: int64(addr), Value: mem[addr]}}
	}
	return widemathNativeByteAccess(mem, pc, r)
}

func TestLizpTokenizerIndependentOracleControls(t *testing.T) {
	for _, c := range []struct {
		src string
		tok byte
		pos int
	}{{"123abc", 4, 3}, {"-12", 3, 3}, {";only", 0, 5}, {"\"open", 5, 5}, {"(x", 1, 1}, {"a\x00b", 3, 1}} {
		tok, pos := lizpTokenizerModel([]byte(c.src), 0)
		if tok != c.tok || pos != c.pos {
			t.Fatal(c, tok, pos)
		}
	}
	good := []mir2.VMDataAccess{{Op: mir2.OpLoad, Addr: 0xc000, Value: 'a'}, {Op: mir2.OpStore, Addr: 0xbff0, Value: 1}, {Op: mir2.OpStore, Addr: 0xbff1, Value: 0}, {Op: mir2.OpLoad, Addr: 0xc001, Value: ' '}}
	if err := lizpTokenizerCheckWrites(good, "next_token", 0, 1); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"swapped-word-bytes", "missing-high-write", "wrong-position", "reversed-source-read"} {
		bad := append([]mir2.VMDataAccess(nil), good...)
		switch name {
		case "swapped-word-bytes":
			bad[1], bad[2] = bad[2], bad[1]
		case "missing-high-write":
			bad = append(bad[:2], bad[3:]...)
		case "wrong-position":
			bad[1].Value = 2
		case "reversed-source-read":
			bad[0].Addr = 0xc001
			bad[3].Addr = 0xc000
		}
		if err := lizpTokenizerCheckWrites(bad, "next_token", 0, 1); err == nil {
			t.Fatal("accepted", name, bad)
		}
	}
}
