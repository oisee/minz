package pipeline

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPFCCOMeasuredNativePacket(t *testing.T) {
	path := "../../../examples/nanz/pfcco_weighted_packet.nanz"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	factory := func() (*hir.Module, error) { return nanz.Parse(string(source), path) }
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{}, Globals: map[string][]byte{"packet_trace": {250, 255}}}
	for _, pair := range [][2]int64{{0, 0}, {0, 1}, {1, 0}, {1, 1}, {25, 26}, {26, 25}, {127, 128}, {128, 127}, {65535, 65535}, {65535, 0}, {0, 65535}, {9, 10}, {10, 9}, {32767, 32768}, {32768, 32767}, {65535, 1}, {1, 65535}} {
		a, b := pair[0], pair[1]
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "weighted_packet", Arguments: []hir.ProfileArgument{{Value: &a}, {Value: &b}}})
	}
	dir := os.Getenv("MINZ_PFCCO_NATIVE_EVIDENCE")
	save := func(name string, v any) {
		if dir == "" {
			return
		}
		var data []byte
		switch v := v.(type) {
		case []byte:
			data = v
		case string:
			data = []byte(v)
		default:
			data, err = json.MarshalIndent(v, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	save("source.nanz", source)
	save("workload.json", w)
	original, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := hir.RunLoopProfile(original, w)
	if err != nil {
		t.Fatal(err)
	}
	trace := uint16(65530)
	for i, c := range w.Calls {
		a, b := *c.Arguments[0].Value, *c.Arguments[1].Value
		want := int64((uint16(b-a) + uint16(a-b)*3) & 65535)
		trace += 2
		if raw.Results[i][0].I != want || !reflect.DeepEqual(raw.Snapshots[i].Globals["packet_trace"], []byte{byte(trace), byte(trace >> 8)}) {
			t.Fatalf("independent arithmetic/state oracle at call %d", i)
		}
	}
	save("raw.json", raw)
	var baselineT uint64
	// This is a Speed search. Report binary size, but do not impose a byte
	// penalty: Size is a separate optimisation goal.
	for _, mode := range []string{"off", "greedy", "pbqp"} {
		h, err := factory()
		if err != nil {
			t.Fatal(err)
		}
		steps, err := CompileHIRSteps(h, Options{ContractOpt: mode != "off", ContractStrategy: map[string]string{"pbqp": "pbqp"}[mode], AssertMode: "none"})
		if err != nil {
			t.Fatal(err)
		}
		m := steps.MIR2Module
		var choices mir2.ContractSet
		if mode != "off" {
			choices = mir2.ContractSet{}
			for _, f := range m.Funcs {
				c := &mir2.ContractChoice{}
				for _, p := range f.Contract.Params {
					c.ParamClasses = append(c.ParamClasses, p.Class)
				}
				for _, r := range f.Contract.Returns {
					c.ReturnClasses = append(c.ReturnClasses, r.Class)
				}
				choices[f.Name] = c
			}
		}
		if choices != nil {
			for _, f := range m.Funcs {
				for i, r := range f.Contract.Returns {
					if choices[f.Name].ReturnClasses[i] != r.Class {
						t.Fatal("return choice not applied")
					}
				}
				for i, p := range f.Contract.Params {
					if choices[f.Name].ParamClasses[i] != p.Class {
						t.Fatal("choice not applied")
					}
				}
			}
		}
		vm := mir2.NewVM(m)
		vm.WriteHeapSlice(vm.GlobalAddr("packet_trace"), []byte{250, 255})
		for i, c := range w.Calls {
			got, e := vm.Call("weighted_packet", []mir2.Value{{I: *c.Arguments[0].Value}, {I: *c.Arguments[1].Value}})
			if e != nil || !reflect.DeepEqual(got, raw.Results[i]) {
				t.Fatalf("%s MIR oracle call %d got %v error %v", mode, i, got, e)
			}
			if !reflect.DeepEqual(vm.ReadHeap(vm.GlobalAddr("packet_trace"), 2), raw.Snapshots[i].Globals["packet_trace"]) {
				t.Fatal("MIR global state mismatch")
			}
		}
		ar := steps.Allocation
		asm := steps.Assembly
		if strings.Count(asm, "CALL displacement") != 2 {
			t.Fatal("fixture must keep two real helper calls")
		}
		run, err := RunZ80ProfileWorkload(steps, original, w, *raw)
		if err != nil {
			t.Fatal(mode, err)
		}
		bin, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
		if err != nil || len(bin.Errors) > 0 {
			t.Fatal(err)
		}
		if mode == "off" {
			baselineT = uint64(run.TotalTStates)
		} else if uint64(run.TotalTStates) >= baselineT {
			// Later backend improvements can benefit the off baseline more than
			// a different register contract. Preserve the semantic oracle and report
			// the loss; a heuristic is not guaranteed to win on every frozen input.
			t.Logf("%s has no Speed benefit: %d T against %d T", mode, run.TotalTStates, baselineT)
		}
		save(mode+"-metrics.txt", fmt.Sprintf("%d T-states / %d binary bytes\n", run.TotalTStates, len(bin.Binary)))
		save(mode+"-choices.json", choices)
		save(mode+"-asm.a80", asm)
		save(mode+"-binary.bin", bin.Binary)
		save(mode+"-mir2.txt", m.Dump())
		save(mode+"-allocation.json", ar)
		save(mode+"-run.json", run)
		t.Logf("%s: %dT/%dB choices=%v", mode, run.TotalTStates, len(bin.Binary), choices)
	}
	t.Log("independent displacement arithmetic and global state verified")
}
