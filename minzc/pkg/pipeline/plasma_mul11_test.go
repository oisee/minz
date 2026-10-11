package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/remogatto/z80"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Exact original plasma_attr. Book v7 numeric u8 wrapping; source glsl_sin is
// an integer quarter-wave approximation, not mathematical floating point sine.
func TestOriginalPlasmaAttributeMul11(t *testing.T) {
	const path = "../../../examples/zx/plasma.nanz"
	src, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	parse := func() *hir.Module {
		h, e := nanz.Parse(string(src), path)
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	save := func(name string, v any) {
		dir := os.Getenv("MINZ_PLASMA_MUL11_ARTIFACTS")
		if dir == "" {
			return
		}
		p := filepath.Join(dir, name)
		if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			t.Fatal(e)
		}
		var data []byte
		switch x := v.(type) {
		case string:
			data = []byte(x)
		case []byte:
			data = x
		default:
			data, e = json.MarshalIndent(v, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
		}
		if e := os.WriteFile(p, data, 0644); e != nil {
			t.Fatal(e)
		}
	}
	sin := func(a byte) byte {
		q := a & 63
		oct := a >> 6
		param := q
		if oct == 1 || oct == 3 {
			param = 63 - q
		}
		d := uint16(64 - param)
		corr := byte((d * d * 7) >> 8)
		if oct < 2 {
			return 240 - corr
		}
		return corr + 16
	}
	model := func(cx, cy byte) int64 {
		wave := sin(cx*8) + sin(cy*11+64)
		color := wave >> 5
		if wave > 210 {
			color |= 0x40
		}
		return int64(color)
	}
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000000, MaxMemory: 65536, ExactDataAccess: false}
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "plasma_attr", Arguments: []hir.ProfileArgument{workloadLiteral(int64(x)), workloadLiteral(int64(y))}})
		}
	}
	for y := 0; y < 256; y++ {
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "plasma_attr", Arguments: []hir.ProfileArgument{workloadLiteral(7), workloadLiteral(int64(y))}})
	}
	save("source.nanz", src)
	save("workload.json", w)

	h := parse()
	rawVM := mir2.NewVM(hir.LowerModule(h))
	rawVM.MaxSteps = 10000000
	var rawResults [][]mir2.Value
	for i, c := range w.Calls {
		want := model(byte(*c.Arguments[0].Value), byte(*c.Arguments[1].Value))
		r, e := rawVM.Call("plasma_attr", []mir2.Value{{I: *c.Arguments[0].Value}, {I: *c.Arguments[1].Value}})
		if e != nil || len(r) != 1 || r[0].I != want {
			t.Fatal("raw independent model", i, r, want, e)
		}
		rawResults = append(rawResults, r)
	}
	save("raw-results.json", rawResults)
	s, e := CompileHIRSteps(parse(), Options{AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if e != nil || len(a.Errors) > 0 {
		t.Fatal(e, a.Errors)
	}
	save("final.mir2", s.MIR2Module.Dump())
	save("target.a80", s.Assembly)
	save("target.bin", a.Binary)
	finalVM := mir2.NewVM(s.MIR2Module)
	finalVM.MaxSteps = 10000000
	mem := &memoryHostZ80Memory{}
	copy(mem.data[0x8000:], a.Binary)
	cpu := z80.NewZ80(mem, memoryHostNoPorts{})
	mem.cpu = cpu
	f := s.MIR2Module.FuncByName("plasma_attr")
	var total int64
	var costs []int64
	for i, c := range w.Calls {
		args := []mir2.Value{{I: *c.Arguments[0].Value}, {I: *c.Arguments[1].Value}}
		r, e := finalVM.Call("plasma_attr", args)
		if e != nil || !reflect.DeepEqual(r, rawResults[i]) {
			t.Fatal("final VM", i, r, e)
		}
		before := mem.data
		mem.data[0xeff0], mem.data[0xeff1] = 0, 3
		cpu.SetSP(0xeff0)
		cpu.SetPC(uint16(a.Symbols["plasma_attr"]))
		for j, p := range f.Contract.Params {
			if e := memoryHostSetArgument(mem, assertParamLocation(p, s.Allocation), args[j].I); e != nil {
				t.Fatal(e)
			}
		}
		start := cpu.Tstates
		for n := 0; n < 10000 && cpu.PC() != 0x300; n++ {
			cpu.DoOpcode()
		}
		if cpu.PC() != 0x300 || cpu.SP() != 0xeff2 || int64(cpu.A) != rawResults[i][0].I {
			t.Fatal("native model/return", i, cpu.A, cpu.PC(), cpu.SP(), rawResults[i])
		}
		cost := int64(cpu.Tstates - start)
		costs = append(costs, cost)
		total += cost
		for addr := range mem.data {
			if addr >= 0xe000 && addr <= 0xeff1 {
				continue
			}
			if mem.data[addr] != before[addr] {
				t.Fatal("unrelated persistent RAM", i, addr)
			}
		}
		if !bytes.Equal(mem.data[0x8000:0x8000+len(a.Binary)], a.Binary) {
			t.Fatal("code corruption")
		}
	}
	save("native.json", map[string]any{"calls": len(w.Calls), "t_states": total, "per_call": costs, "bytes": len(a.Binary), "sha256": fmt.Sprintf("%x", sha256.Sum256(a.Binary))})
	t.Logf("1024 exact original plasma_attr calls; %dT %dB SHA%x", total, len(a.Binary), sha256.Sum256(a.Binary))
}
