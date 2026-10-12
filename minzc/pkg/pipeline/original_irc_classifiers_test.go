package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

func TestOriginalIRCClassifiers(t *testing.T) {
	src, e := os.ReadFile("../../../examples/nanz/irc_client.nanz")
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile("../../../reports/irc-classifiers/freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	var freeze struct {
		SHA          string            `json:"source_sha256"`
		Dependencies map[string]string `json:"dependencies"`
		Lines        []string          `json:"irc_lines"`
	}
	if e = json.Unmarshal(data, &freeze); e != nil || freeze.SHA != fmt.Sprintf("%x", sha256.Sum256(src)) {
		t.Fatal("freeze", e)
	}
	for p, h := range freeze.Dependencies {
		b, e := os.ReadFile("../../../" + p)
		if e != nil || fmt.Sprintf("%x", sha256.Sum256(b)) != h {
			t.Fatal("dependency", p, e)
		}
	}
	save := func(n string, v any) {
		if dir := os.Getenv("MINZ_IRC_CLASSIFIER_ARTIFACTS"); dir != "" {
			var b []byte
			switch x := v.(type) {
			case string:
				b = []byte(x)
			case []byte:
				b = x
			default:
				b, _ = json.MarshalIndent(v, "", "  ")
			}
			p := filepath.Join(dir, n)
			os.MkdirAll(filepath.Dir(p), 0755)
			if e := os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	parse := func() *hir.Module {
		h, e := nanz.ParseWithOpts(string(src), "irc_client.nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
		if e != nil {
			t.Fatal(e)
		}
		return h
	}

	type test struct {
		Fn     string
		Args   []int64
		Input  []byte
		Target []byte
		Want   int64
		Reads  []ircClassifierRead
		User   bool
	}
	model := func(buf, target []byte, offset, n int, zero bool) (int64, []ircClassifierRead) {
		var es []ircClassifierRead
		for i := 0; zero || i < n; i++ {
			if zero {
				es = append(es, ircClassifierRead{"target", i, target[i]})
				if target[i] == 0 {
					return 1, es
				}
			}
			es = append(es, ircClassifierRead{"input", offset + i, buf[offset+i]}, ircClassifierRead{"target", i, target[i]})
			if buf[offset+i] != target[i] {
				return 0, es
			}
		}
		return 1, es
	}
	var cases []test
	for _, line := range freeze.Lines {
		for _, pos := range []int{0, 1, 3, 6, 11, 255} {
			b := bytes.Repeat([]byte{0x5a}, 512)
			copy(b, []byte(line))
			b[len(line)] = 0
			want := int64(6)
			var es []ircClassifierRead
			ping := append([]byte("PING"), 0)
			v, r := model(b, ping, 0, 0, true)
			for i := range r {
				if r[i].Object == "target" {
					r[i].Object = "PING"
				}
			}
			es = append(es, r...)
			if v == 1 {
				want = 0
			} else {
				for i, s := range []string{"PRIVMSG", "JOIN", "PART", "QUIT"} {
					v, r = model(b, []byte(s), pos, len(s), false)
					for j := range r {
						if r[j].Object == "target" {
							r[j].Object = s
						}
					}
					es = append(es, r...)
					if v == 1 {
						want = int64(i + 1)
						break
					}
				}
				if want == 6 {
					es = append(es, ircClassifierRead{"input", pos, b[pos]})
					if b[pos] >= 48 {
						es = append(es, ircClassifierRead{"input", pos, b[pos]})
						if b[pos] <= 57 {
							want = 5
						}
					}
				}
			}
			cases = append(cases, test{Fn: "classify_irc", Args: []int64{0x9000, int64(pos)}, Input: b, Want: want, Reads: es})
		}
	}
	for _, n := range []int{0, 1, 4, 127, 255} {
		for x := 0; x < 256; x++ {
			target := make([]byte, n+1)
			buf := bytes.Repeat([]byte{0x5a}, 512)
			for i := 0; i < n; i++ {
				target[i] = byte(1 + i%254)
				buf[i] = target[i]
			}
			if n > 0 {
				buf[0] = byte(x)
			}
			want, reads := model(buf, target, 0, 0, true)
			cases = append(cases, test{Fn: "buf_starts_with", Args: []int64{0x9000, 0x9400}, Input: buf, Target: target, Want: want, Reads: reads})
		}
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		raw := hir.LowerModule(parse())
		s, e := CompileHIRSteps(parse(), Options{AssertMode: "none", UseHIRGrace: mode != "ordinary", UseGrace: mode != "ordinary", OptSize: mode == "grace-size", ContractOpt: true})
		if e != nil {
			t.Fatal(e)
		}
		a, e := z80asm.NewAssembler().AssembleString("ORG 0x4000\n" + s.Assembly)
		if e != nil || len(a.Errors) > 0 {
			t.Fatal(e, a.Errors)
		}
		save(mode+"/raw.mir", raw.Dump())
		save(mode+"/final.mir", s.MIR2Module.Dump())
		save(mode+"/target.a80", s.Assembly)
		save(mode+"/target.bin", a.Binary)
		z := emulator.NewRemogattoZ80WithScreen()
		z.SetROMEnd(0)
		mem := z.Memory()
		base := bytes.Repeat([]byte{0xa5}, 65536)
		copy(base[0x4000:], a.Binary)
		vms := []*mir2.VM{mir2.NewVM(raw), mir2.NewVM(s.MIR2Module)}
		for _, vm := range vms {
			for i := 0; i < vm.Module.Strings.Len(); i++ {
				vm.ResolveSymbol(fmt.Sprintf("@mir2.str.%d", i))
			}
			vm.EnsureHeap(65536)
		}
		// String pool uses source encodings; model follows the current literal ABI
		// explicitly, never a made-up host read. Named constants are located from
		// actual pool payloads but their expected bytes come from frozen text.
		vmNames := []map[string]int64{}
		nativeNames := map[string]int64{"input": 0x9000, "target": 0x9400}
		for _, vm := range vms {
			names := map[string]int64{"input": 0x9000, "target": 0x9400}
			for i := 0; i < vm.Module.Strings.Len(); i++ {
				p, _ := vm.ResolveSymbol(fmt.Sprintf("@mir2.str.%d", i))
				for _, name := range []string{"PING", "PRIVMSG", "JOIN", "PART", "QUIT"} {
					if vm.Module.Strings.At(i) == name {
						names[name] = p
					}
				}
			}
			vmNames = append(vmNames, names)
		}
		for i := 0; i < s.MIR2Module.Strings.Len(); i++ {
			name := s.MIR2Module.Strings.At(i)
			for _, literal := range []string{"PING", "PRIVMSG", "JOIN", "PART", "QUIT"} {
				if name == literal {
					symbol := fmt.Sprintf("_mir2_str_%d", i)
					p, ok := a.Symbols[symbol]
					if !ok {
						t.Fatal("missing literal", symbol)
					}
					nativeNames[literal] = int64(p)
				}
			}
		}

		normalize := func(v mir2.VMDataAccess, names map[string]int64) (ircClassifierRead, bool) {
			for _, name := range []string{"input", "target", "PING", "PRIVMSG", "JOIN", "PART", "QUIT"} {
				p, ok := names[name]
				if !ok {
					continue
				}
				n := len(name)
				if name == "input" {
					n = 512
				}
				if name == "target" {
					n = 256
				}
				if name == "PING" {
					n++
				}
				if v.Addr >= p && v.Addr < p+int64(n) {
					return ircClassifierRead{name, int(v.Addr - p), v.Value}, true
				}
			}
			return ircClassifierRead{}, false
		}
		public := func(es []ircClassifierRead) []ircClassifierRead {
			var out []ircClassifierRead
			for _, r := range es {
				if r.Object == "input" || r.Object == "target" || r.Object == "user" {
					out = append(out, r)
				}
			}
			return out
		}
		totals := map[string]int{"raw_public_reads": 0, "final_public_reads": 0, "native_public_reads": 0}
		checkTrace := func(events, model []ircClassifierRead, exact bool) {
			if e := ircCheckClassifierTrace(public(events), public(model), exact); e != nil {
				t.Fatal(mode, e)
			}
		}

		allowedFns := map[string]bool{"classify_irc": true, "classify_user_cmd": true, "buf_starts_with": true, "region_match": true}
		fetchAllowed := make([]bool, 65536)
		private := map[int64]bool{}
		for _, l := range a.Listing {
			if len(l.Label) > 7 && l.Label[:7] == "_spill_" {
				for j := range l.Bytes {
					private[int64(l.Address+j)] = true
				}
			}
		}
		for _, f := range s.MIR2Module.Funcs {
			if !allowedFns[f.Name] {
				continue
			}
			lo, ok := a.Symbols[f.Name]
			if !ok {
				t.Fatal("missing native entry", f.Name)
			}
			hi := 0x4000 + len(a.Binary)
			for _, other := range s.MIR2Module.Funcs {
				if p, ok := a.Symbols[other.Name]; ok && p > lo && p < hi {
					hi = p
				}
			}
			for p := lo; p < hi; p++ {
				fetchAllowed[p] = true
			}
			for _, b := range f.Blocks {
				for _, i := range b.Insts {
					if i.Op == mir2.OpCall && !allowedFns[i.Sym] {
						t.Fatal("unexpected pure closure call", f.Name, i.Sym)
					}
					if i.Op == mir2.OpCallIndirect || i.Op == mir2.OpAsm {
						t.Fatal("opaque pure closure", f.Name)
					}
				}
			}
		}
		for _, vm := range vms {
			for _, f := range vm.Module.Funcs {
				if len(f.Blocks) == 0 {
					name := f.Name
					vm.Hosts[name] = func([]mir2.Value) ([]mir2.Value, error) {
						return nil, fmt.Errorf("forbidden network/terminal hook %s", name)
					}
				}
			}
		}

		count := 0
		check := func(c test) {
			count++
			for j, vm := range vms {
				vm.WriteHeapSlice(0x9000, c.Input)
				vm.WriteHeapSlice(0x9400, c.Target)
				if c.User {
					p, _ := vm.ResolveSymbol("usr_buf")
					vm.WriteHeapSlice(p, c.Input)
				}
				before := append([]byte(nil), vm.ReadHeap(0, 65536)...)
				var events []ircClassifierRead
				vm.ObserveDataAccess = func(v mir2.VMDataAccess) {
					if v.Op != mir2.OpLoad {
						t.Fatal("unexpected VM write", mode, c.Fn, v)
					}
					observed := false
					if c.User {
						p, _ := vm.ResolveSymbol("usr_buf")
						if v.Addr >= p && v.Addr < p+256 {
							events = append(events, ircClassifierRead{"user", int(v.Addr - p), v.Value})
							observed = true
						}
					} else if r, ok := normalize(v, vmNames[j]); ok {
						events = append(events, r)
						observed = true
					}
					if !observed {
						t.Fatal("unbound VM data read", mode, c.Fn, j, v, vmNames[j])
					}
				}
				args := []mir2.Value{}
				for _, v := range c.Args {
					args = append(args, mir2.Value{I: v})
				}
				got, e := vm.Call(c.Fn, args)
				if e != nil || len(got) != 1 || got[0].I != c.Want {
					t.Fatal("VM", mode, c.Fn, c.Args, got, c.Want, e)
				}
				checkTrace(events, c.Reads, j == 0)
				key := "raw_public_reads"
				if j == 1 {
					key = "final_public_reads"
				}
				totals[key] += len(public(events))

				if !bytes.Equal(before, vm.ReadHeap(0, 65536)) {
					t.Fatal("VMRAM")
				}
			}
			copy(mem, base)
			copy(mem[0x9000:], c.Input)
			copy(mem[0x9400:], c.Target)
			if c.User {
				copy(mem[a.Symbols["usr_buf"]:], c.Input)
			}
			mem[0xeff0], mem[0xeff1] = 0, 0x70
			before := append([]byte(nil), mem...)
			r := emulator.Registers{PC: uint16(a.Symbols[c.Fn]), SP: 0xeff0, A: 0x42, BC: 0x5555, DE: 0x6666, HL: 0x7777}
			f := s.MIR2Module.FuncByName(c.Fn)
			for i, v := range c.Args {
				loc := assertParamLocation(f.Contract.Params[i], s.Allocation)
				switch loc.Name {
				case "HL":
					r.HL = uint16(v)
				case "DE":
					r.DE = uint16(v)
				case "BC":
					r.BC = uint16(v)
				case "IX":
					r.IX = uint16(v)
				case "IY":
					r.IY = uint16(v)
				case "A":
					r.A = byte(v)
				case "B":
					r.BC = r.BC&255 | uint16(byte(v))<<8
				case "C":
					r.BC = r.BC&0xff00 | uint16(byte(v))
				default:
					t.Fatal("ABI", loc)
				}
			}
			z.SetRegisters(r)
			minSP := uint16(0xeff0)
			done := false
			var events []ircClassifierRead
			for i := 0; i < 100000; i++ {
				if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
					done = true
					break
				}
				if !fetchAllowed[z.GetPC()] {
					t.Fatal("forbidden native function/runtime fetch", mode, c.Fn, z.GetPC())
				}
				for _, v := range lizpTokenizerNativeAccess(mem, z.GetPC(), z.GetRegisters()) {
					if private[v.Addr] {
						continue
					}
					if v.Op != mir2.OpLoad {
						t.Fatal("unexpected native write", mode, c.Fn, v)
					}
					observed := false
					if c.User {
						p := int64(a.Symbols["usr_buf"])
						if v.Addr >= p && v.Addr < p+256 {
							events = append(events, ircClassifierRead{"user", int(v.Addr - p), v.Value})
							observed = true
						}
					} else if r, ok := normalize(v, nativeNames); ok {
						events = append(events, r)
						observed = true
					}
					if !observed {
						t.Fatal("unbound native data read", mode, c.Fn, v)
					}
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
				if z.GetSP() < 0xe000 || z.GetSP() > 0xeff2 {
					t.Fatal("SP")
				}
			}
			if !done || int64(z.GetRegisters().A) != c.Want {
				t.Fatal("native", mode, c.Fn, c.Args, z.GetRegisters(), c.Want)
			}
			checkTrace(events, c.Reads, false)
			totals["native_public_reads"] += len(public(events))
			copy(before[minSP:0xeff0], mem[minSP:0xeff0])
			for _, l := range a.Listing {
				if len(l.Label) > 7 && l.Label[:7] == "_spill_" {
					for j := range l.Bytes {
						before[l.Address+j] = mem[l.Address+j]
					}
				}
			}
			if !bytes.Equal(before, mem) {
				t.Fatal("nativeRAM")
			}
		}
		for _, c := range cases {
			check(c)
		}
		for x := 0; x < 256; x++ {
			for y := 0; y < 256; y++ {
				b := bytes.Repeat([]byte{0x59}, 256)
				b[0], b[1] = byte(x), byte(y)
				want := int64(0)
				reads := []ircClassifierRead{{"user", 0, byte(x)}}
				if x == 47 {
					reads = append(reads, ircClassifierRead{"user", 1, byte(y)})
					want = 4
					switch y {
					case 106:
						want = 1
					case 110:
						want = 2
					case 113:
						want = 3
					}
					if y != 106 {
						reads = append(reads, ircClassifierRead{"user", 1, byte(y)})
					}
					if y != 106 && y != 110 {
						reads = append(reads, ircClassifierRead{"user", 1, byte(y)})
					}
				}
				check(test{Fn: "classify_user_cmd", Input: b, Want: want, Reads: reads, User: true})
			}
		}
		save(mode+"/summary.json", map[string]any{"calls": count, "result": "PASS", "observed_reads": totals, "scope": "3originalclassifiers+region_matchdependency, no network/TUI/main"})
		t.Log(mode, count, "raw/final/native ordered reads fullRAM PC/SP PASS")
	}
}

// Symbolic object names retain relocation-independent identity and byte order.
type ircClassifierRead struct {
	Object string
	Offset int
	Value  byte
}

func ircCheckClassifierTrace(events, model []ircClassifierRead, exact bool) error {
	if exact {
		if fmt.Sprint(events) != fmt.Sprint(model) {
			return fmt.Errorf("raw exact multiplicity: %v vs %v", events, model)
		}
		return nil
	}
	// Only immutable repeat deletion; never an added, reordered, or missing
	// distinct observation. In particular a nonconsecutive repeat isn't collapsed
	// before testing that the full actual sequence is a source subsequence.
	at := 0
	for _, r := range events {
		for at < len(model) && model[at] != r {
			at++
		}
		if at == len(model) {
			return fmt.Errorf("not source subsequence: %v vs %v", events, model)
		}
		at++
	}
	distinct := func(es []ircClassifierRead) []ircClassifierRead {
		var out []ircClassifierRead
		seen := map[ircClassifierRead]bool{}
		for _, r := range es {
			if !seen[r] {
				out = append(out, r)
				seen[r] = true
			}
		}
		return out
	}
	if fmt.Sprint(distinct(events)) != fmt.Sprint(distinct(model)) {
		return fmt.Errorf("missing/reordered distinct reads: %v vs %v", events, model)
	}
	return nil
}
func TestIRCClassifierTraceOracleControls(t *testing.T) {
	a := ircClassifierRead{"target", 0, 1}
	b := ircClassifierRead{"input", 0, 1}
	c := ircClassifierRead{"input", 1, 2}
	model := []ircClassifierRead{a, b, a, c}
	if e := ircCheckClassifierTrace([]ircClassifierRead{a, b, c}, model, false); e != nil {
		t.Fatal("valid immutable forwarding", e)
	}
	if e := ircCheckClassifierTrace(model, model, true); e != nil {
		t.Fatal(e)
	}
	for _, bad := range [][]ircClassifierRead{{a, b, c}, {b, a, c}, {a, b, a, c, a}, {a, b, a}, {a, c, b}, {a, b, {"input", 2, 2}, c}} {
		exact := fmt.Sprint(bad) == fmt.Sprint([]ircClassifierRead{a, b, c})
		if e := ircCheckClassifierTrace(bad, model, exact); e == nil {
			t.Fatal("accepted bad trace", bad, exact)
		}
	}
}
