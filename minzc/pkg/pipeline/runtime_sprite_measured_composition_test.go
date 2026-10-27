package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
)

const runtimeSpriteMeasuredSource = `import hw.sprite
fun paint(frame:^u8) { @sprite() {
 destination "linear_relative" "frame" 6 1 6
 region 0 0 6 1
 row "fff00faa5500" "00050340aaff"
} }
fun is_visible(value:u8)->u8 { if value==0 {return 0} if value==255 {return 0} return 1 }
fun render(frame:^u8,value:u8) { if is_visible(value)!=0 {paint(frame)} }
`

func runtimeSpriteFragmentPool(t *testing.T, p *vir.LocalTableProblem) *vir.RegAllocTable {
	t.Helper()
	widths := map[int]int{}
	for _, op := range p.Ops {
		if op.Dst > 0 {
			widths[op.Dst] = op.Width
		}
	}
	assignment := map[int]int{}
	for v, width := range widths {
		name := "A"
		if width == 16 {
			name = "HL"
		}
		assignment[v] = vir.Z80.LocByName(name)
	}
	for v, loc := range p.Options.ParamLocs {
		assignment[v] = loc
	}
	// A protected base copy uses DE, independently of whether the incoming ABI
	// base is HL or DE. Admission proves equality and survival of live copies.
	for _, op := range p.Ops {
		if op.Op == vir.OpMove && op.Width == 16 {
			assignment[op.Dst] = vir.Z80.LocByName("DE")
		}
	}
	pool := &vir.RegAllocTable{}
	pool.Init()
	begin := 0
	for i, op := range p.Ops {
		if op.Op != vir.OpStore && op.Op != vir.OpRet {
			continue
		}
		if op.Op == vir.OpStore && i+1 < len(p.Ops) && p.Ops[i+1].Op == vir.OpRet {
			continue
		}
		segment := p.Ops[begin : i+1]
		proposal := map[int]int{}
		for _, node := range segment {
			for _, v := range append([]int{node.Dst}, node.Src[:]...) {
				if v > 0 {
					proposal[v] = assignment[v]
				}
			}
		}
		options := p.Options
		options.ParamLocs = map[int]int{}
		for v, loc := range p.Options.ParamLocs {
			if _, exists := proposal[v]; exists {
				options.ParamLocs[v] = loc
			}
		}
		if e := pool.AddChecked(segment, proposal, vir.Z80, options); e != nil {
			t.Fatal("fragment", begin, i, e)
		}
		begin = i + 1
	}
	if begin != len(p.Ops) {
		t.Fatal("incomplete sprite fragments")
	}
	return pool
}

func runtimeSpriteWorkload(heldout bool) hir.ProfileWorkload {
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{}}
	for i := 0; i < 3; i++ {
		data := make([]byte, 32)
		for j := range data {
			if heldout {
				data[j] = byte(255 - j*17 - i*31)
			} else {
				data[j] = byte(j*37 + i*11)
			}
		}
		w.Resources[fmt.Sprint("frame", i)] = data
	}
	values := []int64{1, 0, 2, 255, 99, 1}
	offsets := []int{0, 1, 7, 2, 13, 19}
	if heldout {
		values = []int64{0, 254, 255, 128, 17, 2, 1}
		offsets = []int{0, 3, 8, 11, 17, 22, 0}
	}
	for i, value := range values {
		v := value
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "render", Arguments: []hir.ProfileArgument{{Resource: fmt.Sprint("frame", i%3), Offset: offsets[i]}, {Value: &v}}})
	}
	return w
}

func runtimeSpriteIndependentOracle(t *testing.T, w hir.ProfileWorkload, run *hir.ProfileRun) {
	t.Helper()
	memory := map[string][]byte{}
	for name, data := range w.Resources {
		memory[name] = append([]byte(nil), data...)
	}
	if len(run.Snapshots) != len(w.Calls) {
		t.Fatal("missing call boundaries")
	}
	masks, gfx := []byte{255, 240, 15, 170, 85, 0}, []byte{0, 5, 3, 64, 170, 255}
	for i, call := range w.Calls {
		v := *call.Arguments[1].Value
		if v != 0 && v != 255 {
			data := memory[call.Arguments[0].Resource]
			offset := call.Arguments[0].Offset
			for j := range masks {
				data[offset+j] = (data[offset+j] & masks[j]) | gfx[j]
			}
		}
		if !reflect.DeepEqual(memory, run.Snapshots[i].Resources) {
			t.Fatalf("independent sprite oracle: invocation%d", i)
		}
	}
	if !reflect.DeepEqual(memory, run.Resources) {
		t.Fatal("independent final sprite oracle")
	}
}

func TestRuntimeSpriteMeasuredCompositionHeldout(t *testing.T) {
	factory := func() (*hir.Module, error) { return hardwareSpriteParse(t, runtimeSpriteMeasuredSource), nil }
	modes := []Options{}
	tables := []string{}
	root := os.Getenv("MINZ_RUNTIME_SPRITE_MEASURED_EXPORT")
	if root == "" {
		root = t.TempDir()
	}
	if e := os.MkdirAll(root, 0755); e != nil {
		t.Fatal(e)
	}
	save := func(name string, data []byte) {
		t.Helper()
		path := filepath.Join(root, name)
		if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(path, data, 0644); e != nil {
			t.Fatal(e)
		}
	}
	for _, strategy := range []string{"greedy", "pbqp"} {
		for _, grace := range []bool{false, true} {
			// Measured composition compiles the exact requested configuration
			// (the measurement machinery forces DisableVariantSearch), so the
			// ordinary heldout comparator below must request it too — otherwise
			// S1's searched OptSize build wins heldout against a search-free
			// composition and the generalization check compares two different
			// compilation regimes.
			options := Options{OptSize: true, AssertMode: "none", ContractOpt: true, ContractStrategy: strategy, UseGrace: grace, UseHIRGrace: grace, DisableVariantSearch: true}
			s, e := CompileHIRSteps(hardwareSpriteParse(t, runtimeSpriteMeasuredSource), options)
			if e != nil {
				t.Fatal(e)
			}
			p, e := vir.PrepareLocalTableProblem(s.MIR2Module.FuncByName("paint"), s.Allocation)
			if e != nil {
				t.Fatal(e)
			}
			pool := runtimeSpriteFragmentPool(t, p)
			table, notes, e := ComposeCheckedLocalTables(hardwareSpriteParse(t, runtimeSpriteMeasuredSource), options, pool, 64)
			if e != nil || table.Size() == 0 {
				t.Fatal(notes, e)
			}
			prefix := fmt.Sprintf("tables/%s-grace%v/", strategy, grace)
			path := filepath.Join(root, prefix, "table.json")
			if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
				t.Fatal(e)
			}
			if e = table.Save(path); e != nil {
				t.Fatal(e)
			}
			if e = pool.Save(filepath.Join(root, prefix, "pool.json")); e != nil {
				t.Fatal(e)
			}
			data, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			pinned := options
			pinned.CheckedLocalTablePath = path
			pinned.CheckedLocalTableSHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
			modes = append(modes, options, pinned)
			tables = append(tables, path)
		}
	}
	save("source.nanz", []byte(runtimeSpriteMeasuredSource))
	training, heldout := runtimeSpriteWorkload(false), runtimeSpriteWorkload(true)
	for name, w := range map[string]hir.ProfileWorkload{"train.json": training, "heldout.json": heldout} {
		data, e := json.MarshalIndent(w, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		save(name, data)
	}
	for _, goal := range []string{"speed", "size"} {
		t.Run(goal, func(t *testing.T) {
			result, e := MeasureConditionalLayoutModes(factory, "is_visible", training, modes, goal, 4)
			if e != nil {
				t.Fatal(e)
			}
			ev := result.Evidence
			selected := ev.Runs[ev.SelectedMode]
			winner := selected.Candidates[selected.Selected]
			if ev.Options[ev.SelectedMode].CheckedLocalTablePath == "" {
				t.Fatal("winner did not use the composed table configuration")
			}
			baseline := ev.Runs[0].Candidates[0]
			if !bytes.Equal(result.Binary, winner.Binary) || result.Steps.Assembly != winner.Assembly {
				t.Fatal("selected executable drift")
			}
			if len(result.Binary) >= len(baseline.Binary) || winner.Run.TotalTStates >= baseline.Run.TotalTStates {
				t.Fatal("runtime sprite did not improve", len(baseline.Binary), len(result.Binary), baseline.Run.TotalTStates, winner.Run.TotalTStates)
			}
			for mode, run := range ev.Runs {
				for candidate, c := range run.Candidates {
					snapshots := []hir.ProfileMemorySnapshot{}
					for _, invocation := range c.Run.Invocations {
						snapshots = append(snapshots, invocation.Snapshot)
					}
					runtimeSpriteIndependentOracle(t, training, &hir.ProfileRun{Snapshots: snapshots, Resources: c.Run.Resources})
					if goal == "speed" && c.Run.TotalTStates < winner.Run.TotalTStates || goal == "size" && len(c.Binary) < len(result.Binary) {
						t.Fatal("not global measured minimum")
					}
					save(fmt.Sprintf("%s/candidates/mode%d-layout%d.a80", goal, mode, candidate), []byte(c.Assembly))
					save(fmt.Sprintf("%s/candidates/mode%d-layout%d.bin", goal, mode, candidate), c.Binary)
				}
			}
			source, e := factory()
			if e != nil {
				t.Fatal(e)
			}
			oracle, e := hir.RunLoopProfile(source, heldout)
			if e != nil {
				t.Fatal(e)
			}
			runtimeSpriteIndependentOracle(t, heldout, oracle)
			held, e := RunZ80ProfileWorkload(result.Steps, source, heldout, *oracle)
			if e != nil {
				t.Fatal(e)
			}
			ordinary, e := CompileHIRSteps(hardwareSpriteParse(t, runtimeSpriteMeasuredSource), modes[0])
			if e != nil {
				t.Fatal(e)
			}
			ordinaryHeld, e := RunZ80ProfileWorkload(ordinary, source, heldout, *oracle)
			if e != nil {
				t.Fatal(e)
			}
			if held.TotalTStates >= ordinaryHeld.TotalTStates {
				t.Fatal("heldout did not improve", ordinaryHeld.TotalTStates, held.TotalTStates)
			}
			// Re-execute the selected paint body outside the profiling harness,
			// with complete writable RAM and an independent 64-KB oracle.
			binary, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + result.Steps.Assembly)
			if e != nil {
				t.Fatal(e)
			}
			if len(binary.Errors) > 0 {
				t.Fatal(binary.Errors)
			}
			for _, base := range []int{0x4000, 0x60fd, 0xfffd} {
				for salt := 0; salt < 64; salt++ {
					z := emulator.NewRemogattoZ80WithScreen()
					z.SetROMEnd(0)
					for address := range z.Memory() {
						z.Memory()[address] = byte(address*37 + address/256*11 + salt)
					}
					if e = z.LoadMemory(0x8000, binary.Binary); e != nil {
						t.Fatal(e)
					}
					z.Memory()[0xeff0], z.Memory()[0xeff1] = 0, 0x70
					want := append([]byte(nil), z.Memory()...)
					masks, gfx := []byte{255, 240, 15, 170, 85, 0}, []byte{0, 5, 3, 64, 170, 255}
					for i := range masks {
						address := int(uint16(base + i))
						want[address] = (want[address] & masks[i]) | gfx[i]
					}
					regs := emulator.Registers{PC: uint16(binary.Symbols["paint"]), SP: 0xeff0, A: 0xa5, F: byte(salt), HL: 0x1234, BC: 0x5678, DE: 0x9abc, IX: 0x1357, IY: 0x2468}
					fn := result.Steps.MIR2Module.FuncByName("paint")
					runtimeSpriteABI(t, &regs, result.Steps.Allocation.Locs[fn.Contract.Params[0].Reg].Name, uint16(base))
					z.SetRegisters(regs)
					for step := 0; step < 1000 && (z.GetPC() != 0x7000 || z.GetSP() != 0xeff2); step++ {
						z.Step()
					}
					if z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
						t.Fatal("selected paint return", z.GetRegisters())
					}
					if !bytes.Equal(want, z.Memory()) {
						t.Fatalf("selected paint full RAM base%04x salt%d", base, salt)
					}
				}
			}
			save(goal+"/selected.a80", []byte(result.Steps.Assembly))
			save(goal+"/selected.bin", result.Binary)
			data, e := json.MarshalIndent(ev, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
			save(goal+"/evidence.json", data)
			data, e = json.MarshalIndent(map[string]any{"selected_mode": ev.SelectedMode, "selected_layout": selected.Selected, "baseline_bytes": len(baseline.Binary), "selected_bytes": len(result.Binary), "training_baseline_T": baseline.Run.TotalTStates, "training_selected_T": winner.Run.TotalTStates, "heldout_baseline_T": ordinaryHeld.TotalTStates, "heldout_selected_T": held.TotalTStates, "heldout_run": held}, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
			save(goal+"/metrics.json", data)
			t.Logf("%s mode%d layout%d: %d→%d B; train %d→%d T; heldout %d→%d T", goal, ev.SelectedMode, selected.Selected, len(baseline.Binary), len(result.Binary), baseline.Run.TotalTStates, winner.Run.TotalTStates, ordinaryHeld.TotalTStates, held.TotalTStates)
		})
	}
	for _, path := range tables {
		data, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		matched := false
		for _, mode := range modes {
			if mode.CheckedLocalTablePath == path {
				matched = mode.CheckedLocalTableSHA256 == fmt.Sprintf("%x", sha256.Sum256(data))
			}
		}
		if !matched {
			t.Fatal("saved table mutated")
		}
	}
}
