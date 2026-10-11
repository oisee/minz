package pipeline

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

func workloadFixture(t *testing.T, text string, w hir.ProfileWorkload) (Steps, *hir.Module, hir.ProfileRun) {
	t.Helper()
	input, err := nanz.Parse(text, "workload.nanz")
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := nanz.Parse(text, "workload.nanz")
	if err != nil {
		t.Fatal(err)
	}
	oracle, err := hir.RunLoopProfile(input, w)
	if err != nil {
		t.Fatal(err)
	}
	steps, err := CompileHIRSteps(compiled, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	return steps, input, *oracle
}
func workloadLiteral(v int64) hir.ProfileArgument { return hir.ProfileArgument{Value: &v} }
func workloadSpec(calls ...hir.ProfileInvocation) hir.ProfileWorkload {
	return hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Resources: map[string][]byte{}, Calls: calls}
}

func TestZ80ProfileWorkloadPersistentMutations(t *testing.T) {
	const text = `global tally:u8=1
global counters:[u8;3]=[2,3,4]
fun mutate(p:^u8,n:u8)->u8 {
 tally=tally+n
 counters[1]=counters[1]+n
 p[0]=p[0]+n
 return tally+p[0]
}
fun caller(p:^u8,n:u8)->u8 {var v:u8=mutate(p,n)
 return v+1}
fun touch(p:^u8) {p[1]=p[0]
 tally=tally+1}
fun void_caller(p:^u8) {touch(p)}
fun word(n:u16)->u16 {return n+1}
`
	w := workloadSpec(
		hir.ProfileInvocation{Function: "caller", Arguments: []hir.ProfileArgument{{Resource: "buffer"}, workloadLiteral(2)}},
		hir.ProfileInvocation{Function: "caller", Arguments: []hir.ProfileArgument{{Resource: "buffer"}, workloadLiteral(3)}},
		hir.ProfileInvocation{Function: "void_caller", Arguments: []hir.ProfileArgument{{Resource: "buffer"}}},
		hir.ProfileInvocation{Function: "word", Arguments: []hir.ProfileArgument{workloadLiteral(0x1234)}},
	)
	w.Resources = map[string][]byte{"buffer": {10, 20}, "untouched": {33}}
	w.Globals = map[string][]byte{"tally": {5}, "counters": {6, 7, 8}}
	steps, source, oracle := workloadFixture(t, text, w)
	out, err := RunZ80ProfileWorkload(steps, source, w, oracle)
	if err != nil {
		t.Fatalf("%v\n%s", err, steps.Assembly)
	}
	if !strings.Contains(steps.Assembly, "CALL mutate") {
		t.Fatal("fixture must exercise ordinary internal CALL")
	}
	if len(out.Invocations) != 4 || out.Invocations[0].Results[0].I != 20 || out.Invocations[1].Results[0].I != 26 || out.Invocations[3].Results[0].I != 0x1235 {
		t.Fatalf("results: %+v", out.Invocations)
	}
	if len(out.Invocations[2].Results) != 0 || !bytes.Equal(out.Resources["buffer"], []byte{15, 15}) || !bytes.Equal(out.Globals["tally"], []byte{11}) || !bytes.Equal(out.Globals["counters"], []byte{6, 12, 8}) {
		t.Fatalf("state: %+v", out)
	}
	program, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	if out.BinarySHA256 != fmt.Sprintf("%x", sha256.Sum256(program.Binary)) || len(out.BootstrapSHA256) != 4 {
		t.Fatal("digest missing")
	}
	var entryCycles int64
	for _, inv := range out.Invocations {
		if inv.TStates <= 0 {
			t.Fatal("missing timing")
		}
		entryCycles += inv.TStates
	}
	if out.TotalTStates != entryCycles || out.BootstrapTStates <= 0 || out.ExecutedSteps <= 0 {
		t.Fatal("bootstrap budget/timing missing")
	}
	if source.PointerWalkPolicy != nil || source.LoopLineage != nil || w.Resources["buffer"][0] != 10 {
		t.Fatal("input mutated")
	}
	again, err := RunZ80ProfileWorkload(steps, source, w, oracle)
	if err != nil || again.TotalTStates != out.TotalTStates || again.BinarySHA256 != out.BinarySHA256 {
		t.Fatalf("nondeterministic: %v", err)
	}
}

func TestZ80ProfileWorkloadOracleMismatch(t *testing.T) {
	const text = `global total:u8=0
fun add(p:^u8,n:u8)->u8 {total=total+n
 p[0]=p[0]+1
 return total}`
	w := workloadSpec(hir.ProfileInvocation{Function: "add", Arguments: []hir.ProfileArgument{{Resource: "input"}, workloadLiteral(3)}})
	w.Resources = map[string][]byte{"input": {9}}
	for _, kind := range []string{"result", "global", "resource", "actual-binary", "missing-global", "missing-resource"} {
		t.Run(kind, func(t *testing.T) {
			steps, source, oracle := workloadFixture(t, text, w)
			switch kind {
			case "result":
				oracle.Results[0][0].I++
			case "global":
				oracle.Globals["total"][0]++
			case "resource":
				oracle.Resources["input"][0]++
			case "missing-global":
				delete(oracle.Globals, "total")
			case "missing-resource":
				delete(oracle.Resources, "input")
			case "actual-binary":
				steps.Assembly = strings.Replace(steps.Assembly, "add:", "add:\n LD A, 99\n RET\n", 1)
			}
			out, err := RunZ80ProfileWorkload(steps, source, w, oracle)
			if err == nil || out != nil {
				t.Fatalf("accepted mismatch: %v %+v", err, out)
			}
		})
	}
}

func TestZ80ProfileWorkloadCumulativeLimit(t *testing.T) {
	const text = `fun loop(n:u8)->u8 {var i:u8=0
 while i<n {i=i+1}
 return i}`
	w := workloadSpec(hir.ProfileInvocation{Function: "loop", Arguments: []hir.ProfileArgument{workloadLiteral(4)}}, hir.ProfileInvocation{Function: "loop", Arguments: []hir.ProfileArgument{workloadLiteral(4)}})
	steps, source, oracle := workloadFixture(t, text, w)
	full, err := RunZ80ProfileWorkload(steps, source, w, oracle)
	if err != nil {
		t.Fatal(err)
	}
	w.MaxSteps = full.ExecutedSteps
	exactOracle, err := hir.RunLoopProfile(source, w)
	if err != nil {
		t.Fatal(err)
	}
	exact, err := RunZ80ProfileWorkload(steps, source, w, *exactOracle)
	if err != nil || exact.ExecutedSteps != full.ExecutedSteps {
		t.Fatalf("exact boundary failed: %v", err)
	}
	w.MaxSteps = full.ExecutedSteps - 1
	oracleRun, err := hir.RunLoopProfile(source, w)
	if err != nil {
		t.Fatal(err)
	}
	out, err := RunZ80ProfileWorkload(steps, source, w, *oracleRun)
	if out != nil || err == nil || !strings.Contains(err.Error(), "cumulative instruction limit") {
		t.Fatalf("budget bypass: %+v %v", out, err)
	}
}

func TestZ80ProfileWorkloadRefusals(t *testing.T) {
	const text = `fun id(n:u8)->u8 {return n}`
	for _, kind := range []string{"forward-copy-policy", "schema", "limits", "ceiling", "memory", "context", "failed-oracle", "missing-ABI", "SMC", "ASM", "ports", "hosts", "numeric-pointer", "range", "program-stack", "bounds", "code-write", "same-code-write", "compiled-SMC"} {
		t.Run(kind, func(t *testing.T) {
			w := workloadSpec(hir.ProfileInvocation{Function: "id", Arguments: []hir.ProfileArgument{workloadLiteral(3)}})
			steps, source, oracle := workloadFixture(t, text, w)
			switch kind {
			case "forward-copy-policy":
				source.ForwardCopyPolicy = map[string]bool{}
			case "schema":
				w.Schema = "bad"
			case "limits":
				w.MaxSteps = 0
			case "ceiling":
				w.MaxSteps = workloadStepLimit + 1
			case "memory":
				w.MaxMemory = 16
			case "context":
				w.Resources["new"] = []byte{1}
			case "failed-oracle":
				oracle.Profile.FailedCalls = 1
			case "missing-ABI":
				steps.Allocation = nil
			case "SMC":
				source.Funcs[0].Params[0].SMC = true
			case "ASM":
				steps.MIR2Module.Funcs[0].Blocks[0].Insts = append(steps.MIR2Module.Funcs[0].Blocks[0].Insts, &mir2.Inst{Op: mir2.OpAsm})
			case "ports":
				steps.MIR2Module.Funcs[0].Blocks[0].Insts = append(steps.MIR2Module.Funcs[0].Blocks[0].Insts, &mir2.Inst{Op: mir2.OpOut8})
			case "hosts":
				steps.MIR2Module.Funcs[0].Attrs.IsExtern = true
			case "numeric-pointer":
				source.Funcs[0].Params[0].Ty = mir2.TyPtr
			case "range":
				*w.Calls[0].Arguments[0].Value = 256
			case "program-stack":
				steps.Assembly += "\n DS 32768\n"
			case "bounds":
				steps.Assembly = strings.Replace(steps.Assembly, "id:", "id:\n LD A, (0x0050)\n", 1)
			case "code-write":
				steps.Assembly = strings.Replace(steps.Assembly, "id:", "id:\n LD A, 0\n LD (id), A\n", 1)
			case "compiled-SMC":
				steps.Assembly += "\n_tsmc_unused:\n LD HL, 0\n"
			case "same-code-write":
				steps.Assembly = strings.Replace(steps.Assembly, "id:", "id:\n LD A, (id)\n LD (id), A\n", 1)
			}
			out, err := RunZ80ProfileWorkload(steps, source, w, oracle)
			if err == nil || out != nil {
				t.Fatalf("accepted refusal %s: %+v", kind, out)
			}
		})
	}
}

func TestZ80ProfileWorkloadPointerReturnRefused(t *testing.T) {
	text := `fun identity(p:^u8)->^u8 {return p}`
	w := workloadSpec(hir.ProfileInvocation{Function: "identity", Arguments: []hir.ProfileArgument{{Resource: "input"}}})
	w.Resources = map[string][]byte{"input": {1}}
	steps, source, oracle := workloadFixture(t, text, w)
	out, err := RunZ80ProfileWorkload(steps, source, w, oracle)
	if out != nil || err == nil || !strings.Contains(err.Error(), "normalization") {
		t.Fatalf("unsafe pointer equality: %+v %v", out, err)
	}
}

func TestZ80ProfileWorkloadStorageBudgets(t *testing.T) {
	const text = `fun id(n:u8)->u8 {return n}`
	for _, kind := range []string{"memory", "resource-code-overlap", "resource-pointer-literal", "literal-range", "pointer-global", "global-code-overlap", "infinite", "resource-bounds"} {
		t.Run(kind, func(t *testing.T) {
			w := workloadSpec(hir.ProfileInvocation{Function: "id", Arguments: []hir.ProfileArgument{workloadLiteral(3)}})
			steps, source, oracle := workloadFixture(t, text, w)
			switch kind {
			case "memory":
				w.MaxMemory = 16
			case "resource-code-overlap":
				w.Resources["oversized"] = make([]byte, workloadProgram)
				oracle.Resources["oversized"] = make([]byte, workloadProgram)
			case "literal-range":
				*w.Calls[0].Arguments[0].Value = 256
			case "resource-pointer-literal":
				source.Funcs[0].Params[0].Ty = mir2.TyPtr
				steps.MIR2Module.Funcs[0].Contract.Params[0].Ty = mir2.TyPtr
			case "pointer-global":
				source.Globals = append(source.Globals, mir2.Global{Name: "pointer", Ty: mir2.TyPtr, Init: []byte{0, 0}})
			case "global-code-overlap":
				g := mir2.Global{Name: "fake", Ty: mir2.TyU8, Init: []byte{0}}
				source.Globals = append(source.Globals, g)
				steps.MIR2Module.Globals = append(steps.MIR2Module.Globals, g)
				oracle.Globals["fake"] = []byte{0}
				steps.Assembly = "id:\n LD A, 3\n RET\nfake EQU id+1\n"
			case "infinite":
				steps.Assembly = "id:\n JP id\n"
				w.MaxSteps = 17
			case "resource-bounds":
				w.Resources["tiny"] = []byte{2}
				oracle.Resources["tiny"] = []byte{2}
				steps.Assembly = "id:\n LD A, (1025)\n LD A, 3\n RET\n"
			}
			context, err := hir.BuildWorkloadProfileContext(source, w)
			if err != nil {
				t.Fatal(err)
			}
			key, err := hir.BuildProfileCompatibility(source, context)
			if err != nil {
				t.Fatal(err)
			}
			oracle.Profile.Compatibility = key
			out, err := RunZ80ProfileWorkload(steps, source, w, oracle)
			if err == nil || out != nil {
				t.Fatalf("accepted storage/budget refusal %s", kind)
			}
			if kind == "memory" && !strings.Contains(err.Error(), "memory limit") {
				t.Fatal(err)
			}
			if kind == "infinite" && !strings.Contains(err.Error(), "instruction limit") {
				t.Fatal(err)
			}
			if kind == "global-code-overlap" && !strings.Contains(err.Error(), "overlaps code") {
				t.Fatal(err)
			}
		})
	}
}
