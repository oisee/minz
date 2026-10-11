package pipeline

import (
	"bytes"
	"encoding/json"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Inputs fixed before timing either strategy. This checks an existing compiler
// option, not profile selection; input independence does not prove general gain.
var pfccoTokenizerInputs = []string{
	"var buffer_name = 255\nif true { return buffer_name + 1 } // tail\n",
	"alpha alpha beta + 007 <= 9 != 0 / \"literal\"",
}

func TestPFCCOOriginalTokenizerRenderingConsumer(t *testing.T) {
	const path = "../../../examples/nanz/self_tokenizer.nanz"
	src, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var sizes []int
	var renderCosts [][]int
	for _, strategy := range []string{"", "pbqp"} {
		h, e := nanz.Parse(string(src), path)
		if e != nil {
			t.Fatal(e)
		}
		opts := Options{ContractOpt: true, ContractStrategy: strategy, OptSize: true, AssertMode: "none"}
		steps, e := CompileHIRSteps(h, opts)
		if e != nil {
			t.Fatal(e)
		}
		// Fixed extern model for ordinary source. Same adapter for both strategies;
		// all generated caller moves/saves retained. No guarded runtime changes.
		peek, poke := steps.MIR2Module.FuncByName("peek"), steps.MIR2Module.FuncByName("poke")
		if assertParamLocation(peek.Contract.Params[0], steps.Allocation).Name != "HL" || assertParamLocation(poke.Contract.Params[0], steps.Allocation).Name != "HL" || assertParamLocation(poke.Contract.Params[1], steps.Allocation).Name != "C" {
			t.Fatal("extern ABI")
		}
		adapted := strings.Replace(steps.Assembly, "peek:\n", "peek:\n LD A,(HL)\n RET\n", 1)
		adapted = strings.Replace(adapted, "poke:\n", "poke:\n LD (HL),C\n RET\n", 1)
		bin, e := z80asm.NewAssembler().AssembleString("ORG 0x400\n" + adapted)
		if e != nil || len(bin.Errors) > 0 {
			t.Fatal(e, bin.Errors)
		}
		os.WriteFile("/tmp/tokenizer-debug.a80", []byte("ORG 0x400\n"+adapted), 0644)
		plain, e := z80asm.NewAssembler().AssembleString(steps.Assembly)
		if e != nil || len(plain.Errors) > 0 {
			t.Fatal(e, plain.Errors)
		}
		sizes = append(sizes, len(plain.Binary))
		symbol := func(n string) uint16 {
			v, ok := bin.Symbols[n]
			if !ok {
				v, ok = bin.Symbols[strings.ToUpper(n)]
			}
			if !ok {
				t.Fatal("symbol", n)
			}
			return uint16(v)
		}
		initial := map[string][]byte{}
		mutable := map[uint16]bool{}
		for _, g := range h.Globals {
			data := make([]byte, mir2.ByteWidth(g.Ty))
			copy(data, g.Init)
			initial[g.Name] = data
		}
		for _, g := range steps.MIR2Module.Globals {
			if !g.IsConst {
				for i := 0; i < mir2.ByteWidth(g.Ty); i++ {
					mutable[symbol(g.Name)+uint16(i)] = true
				}
			}
		}
		// Exact assembler listing spans for generated spill data only; code and
		// string literals remain immutable. A label prefix never exempts nearby code.
		for _, line := range bin.Listing {
			if strings.HasPrefix(strings.ToLower(line.Label), "_spill_") {
				for j := range line.Bytes {
					mutable[uint16(line.Address+j)] = true
				}
			}
		}
		vm := mir2.NewVM(steps.MIR2Module)
		vm.EnsureHeap(65536)
		vm.Hosts["peek"] = func(a []mir2.Value) ([]mir2.Value, error) {
			return []mir2.Value{{I: int64(vm.ReadHeap(int64(uint16(a[0].I)), 1)[0])}}, nil
		}
		vm.Hosts["poke"] = func(a []mir2.Value) ([]mir2.Value, error) {
			vm.WriteHeap(int64(uint16(a[0].I)), byte(a[1].I))
			return nil, nil
		}
		z := emulator.NewRemogattoZ80WithScreen()
		z.SetROMEnd(0)
		z.LoadMemory(0x400, bin.Binary)
		z.LoadMemory(0xeffe, []byte{0, 0xf1})
		var output []byte
		z.SetIOHandlers(nil, func(p uint16, b byte) {
			if byte(p) != 0x23 {
				t.Errorf("port%x", p)
			}
			output = append(output, b)
		})
		for i := 0; i < 8192; i++ {
			z.SetMemory(0x9000+uint16(i), 0xa5)
			vm.WriteHeap(0x9000+int64(i), 0xa5)
		}
		for i := 0; i < 4096; i++ {
			z.SetMemory(0xb000+uint16(i), 0xc3)
			vm.WriteHeap(0xb000+int64(i), 0xc3)
		}
		for _, p := range []uint16{0x8fff, 0xd000, 0xe000, 0xf000} {
			z.SetMemory(p, 0xa5)
		}
		read := func(p uint16, n int) []byte {
			b := make([]byte, n)
			for i := range b {
				b[i] = z.GetMemory(p + uint16(i))
			}
			return b
		}
		call := func(n string) int {
			t.Helper()
			z.SetRegisters(emulator.Registers{A: 0x67, F: 0x89, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468, PC: symbol(n), SP: 0xeffe})
			start := z.GetCycles()
			for i := 0; i < 3000000 && z.GetRegisters().PC != 0xf100; i++ {
				z.Step()
			}
			r := z.GetRegisters()
			if r.PC != 0xf100 || r.SP != 0xf000 {
				t.Fatalf("%s %s PC%04x SP%04x", strategy, n, r.PC, r.SP)
			}
			return z.GetCycles() - start
		}
		if _, e = vm.Call("init_keywords", nil); e != nil {
			t.Fatal(e)
		}
		initCost := call("init_keywords")
		model := newOriginalTokenizerModel()
		model.init()
		costs := []int{}
		for index, input := range pfccoTokenizerInputs {
			data := make([]byte, 4096)
			copy(data, input)
			z.LoadMemory(0xc000, data)
			vm.WriteHeapSlice(0xc000, data)
			word := []byte{byte(len(input)), byte(len(input) >> 8)}
			z.LoadMemory(symbol("src_len"), word)
			vm.WriteHeapSlice(vm.GlobalAddr("src_len"), word)
			vr, e := vm.Call("tokenize_all", nil)
			if e != nil {
				t.Fatal(e)
			}
			tokenCost := call("tokenize_all")
			model.run(input)
			if len(vr) != 1 || uint16(vr[0].I) != model.count || z.GetRegisters().HL != model.count {
				t.Fatal("count", strategy, index, vr, z.GetRegisters())
			}
			checkState := func() {
				t.Helper()
				for _, region := range []struct {
					p uint16
					b []byte
				}{{0x9000, model.arena}, {0xb000, model.tokens}, {0xc000, data}} {
					if !bytes.Equal(read(region.p, len(region.b)), region.b) || !bytes.Equal(vm.ReadHeap(int64(region.p), len(region.b)), region.b) {
						t.Fatal("region state", strategy, index, region.p)
					}
				}
				for n, b := range model.globals(uint16(len(input)), initial) {
					if !bytes.Equal(read(symbol(n), len(b)), b) || !bytes.Equal(vm.ReadHeap(vm.GlobalAddr(n), len(b)), b) {
						t.Fatal("global", strategy, index, n)
					}
				}
			}
			checkState()
			output = nil
			renderCost := call("print_all_tokens")
			costs = append(costs, renderCost)
			names := map[byte]string{1: "id", 2: "num", 3: "(", 4: ")", 5: "{", 6: "}", 7: ":", 8: ",", 9: "->", 10: "=", 11: "==", 12: "+", 13: "-", 14: "*", 21: "str"}
			var want strings.Builder
			for i := 0; i < int(model.count); i++ {
				tok := model.tokens[i*4 : i*4+4]
				name := names[tok[0]]
				if tok[0] == 30 {
					p := uint16(tok[1]) | uint16(tok[2])<<8
					for k, v := range model.keywords {
						if p == v {
							name = strings.ToLower(strings.TrimPrefix(k, "KW_"))
						}
					}
				}
				want.WriteString(name)
				want.WriteByte(' ')
			}
			if string(output) != want.String() {
				t.Fatalf("stdout strategy%s input%d got%q want%q", strategy, index, output, want.String())
			}
			checkState()
			for i, b := range bin.Binary {
				p := uint16(0x400 + i)
				if !mutable[p] && z.GetMemory(p) != b {
					t.Fatalf("immutable code/literal strategy%s address%04x before%02x after%02x", strategy, p, b, z.GetMemory(p))
				}
			}
			for _, p := range []uint16{0x8fff, 0xd000, 0xe000, 0xf000} {
				if z.GetMemory(p) != 0xa5 {
					t.Fatal("canary", p)
				}
			}
			t.Logf("strategy%q input%d init%dT tokenizer%dT render%dT count%d stdout%q", strategy, index, initCost, tokenCost, renderCost, model.count, output)
			if dir := os.Getenv("PFCCO_TOKENIZER_EXPORT"); dir != "" {
				tag := strategy
				if tag == "" {
					tag = "greedy"
				}
				os.MkdirAll(dir, 0755)
				capture, _ := json.MarshalIndent(map[string]any{"input": input, "stdout": string(output), "init_t": initCost, "tokenize_t": tokenCost, "render_t": renderCost, "globals": model.globals(uint16(len(input)), initial), "strings": read(0x9000, 8192), "tokens": read(0xb000, 4096)}, "", "  ")
				os.WriteFile(filepath.Join(dir, tag+"-input"+string(rune('0'+index))+".json"), capture, 0644)
			}
		}
		renderCosts = append(renderCosts, costs)
		if dir := os.Getenv("PFCCO_TOKENIZER_EXPORT"); dir != "" {
			tag := strategy
			if tag == "" {
				tag = "greedy"
			}
			for n, b := range map[string][]byte{tag + ".a80": []byte(steps.Assembly), tag + ".bin": plain.Binary, tag + "-native.a80": []byte("ORG 0x400\n" + adapted), tag + "-native.bin": bin.Binary, "source.nanz": src} {
				os.WriteFile(filepath.Join(dir, n), b, 0644)
			}
		}
	}
	if sizes[1] >= sizes[0] {
		t.Fatal("no size gain", sizes)
	}
	t.Logf("ordinary fullBIN%d→%dB; renderer costs%v→%v (no assumed speed gain)", sizes[0], sizes[1], renderCosts[0], renderCosts[1])
}
