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

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Program workloads keep every original function, assertion and sandbox. The
// oracle below is host arithmetic/state, never a second execution of source HIR.
type profileProgramCase struct {
	name, source string
	workload     hir.ProfileWorkload
	expected     []int64
	snapshots    []hir.ProfileMemorySnapshot
}

func profileProgramCases() []profileProgramCase {
	makeCase := func(name, source string) profileProgramCase {
		return profileProgramCase{name: name, source: source, workload: hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, Resources: map[string][]byte{}}}
	}
	add := func(c *profileProgramCase, f string, want int64, args ...int64) {
		call := hir.ProfileInvocation{Function: f, Arguments: []hir.ProfileArgument{}}
		for _, a := range args {
			v := a
			call.Arguments = append(call.Arguments, hir.ProfileArgument{Value: &v})
		}
		c.workload.Calls = append(c.workload.Calls, call)
		c.expected = append(c.expected, want)
		c.snapshots = append(c.snapshots, hir.ProfileMemorySnapshot{Globals: map[string][]byte{}, Resources: map[string][]byte{}})
	}
	arena := makeCase("arena", "08_arena_allocator.nanz")

	state := map[string][]byte{}
	// Sentinel initial globals prove that ordered calls share storage and that
	// test_sizeof leaves the pre-existing state intact.
	pack := func(ptr, end uint16) []byte { return []byte{byte(ptr), byte(ptr >> 8), byte(end), byte(end >> 8)} }
	for _, g := range []string{"arena", "perm", "level", "frame"} {
		state[g] = pack(0x1234, 0x5678)
	}
	arena.workload.Globals = state
	snapshot := func() {
		s := &arena.snapshots[len(arena.snapshots)-1]
		for g, b := range state {
			s.Globals[g] = append([]byte{}, b...)
		}
	}
	for _, f := range []string{"test_sizeof", "test_alloc_sequence", "test_oom", "test_small_allocs", "test_split", "test_typed_alloc"} {
		add(&arena, f, 0)
		switch f {
		case "test_alloc_sequence":
			state["arena"] = pack(0xC000+100, 0xFF00)
		case "test_oom":
			state["arena"] = pack(0xC010, 0xC010)
		case "test_small_allocs":
			state["arena"] = pack(0xC004, 0xC400)
		case "test_split":
			state["perm"] = pack(0xC008, 0xC100)
			state["level"] = pack(0xC100+200, 0xC900)
			state["frame"] = pack(0xC910, 0xCD00)
		case "test_typed_alloc":
			state["arena"] = pack(0xC00C, 0xD000)
		}
		snapshot()
	}
	add(&arena, "init_frame", 0x3F00)
	state["arena"] = pack(0xC000, 0xFF00)
	snapshot()
	// Include a deliberately wrong expectation: the allocation still advances.
	for i, offset := range []uint16{0, 64, 129, 192} {
		want := int64(0)
		if i == 2 {
			want = 1
		}
		add(&arena, "alloc_and_check", want, int64(0xC000+offset))
		state["arena"] = pack(uint16(0xC000+(i+1)*64), 0xFF00)
		snapshot()
		add(&arena, "get_remaining", int64(0x3F00-(i+1)*64))
		snapshot()
	}
	// Own initial bytes: later oracle state mutations must not alter workload input.
	arena.workload.Globals = map[string][]byte{}
	for _, g := range []string{"arena", "perm", "level", "frame"} {
		arena.workload.Globals[g] = pack(0x1234, 0x5678)
	}
	sm := makeCase("state-machine", "12_state_machine.nanz")
	// Complete transition matrix for all byte inputs and all declared states.
	for s := int64(0); s < 4; s++ {
		for input := int64(0); input < 256; input++ {
			next := int64(3)
			switch s {
			case 0:
				next = 0
				if input >= 1 && input <= 3 {
					next = input
				}
			case 1:
				next = 1
				if input == 0 || input == 2 || input == 3 {
					next = input
				}
			case 2:
				next = 0
				if input == 3 {
					next = 3
				}
			}
			add(&sm, "next_state", next, s, input)
		}
		alive := int64(1)
		if s == 3 {
			alive = 0
		}
		add(&sm, "is_alive", alive, s)
		add(&sm, "state_speed", []int64{0, 2, 4, 0}[s], s)
	}
	option := makeCase("option", "10_adt_option.nanz")
	for i, f := range []string{"test_unwrap_some", "test_unwrap_none", "test_is_some", "test_is_none", "test_safe_div_ok"} {
		add(&option, f, []int64{42, 77, 1, 0, 3}[i])
	}
	for _, a := range []int64{0, 1, 2, 3, 15, 127, 128, 254, 255} {
		for _, b := range []int64{0, 1, 2, 3, 15, 127, 128, 254, 255} {
			encoded := int64(0) // None: tag 0, payload 0
			if b != 0 {
				encoded = 256 + a/b
			} // Some: tag 1, byte quotient
			add(&option, "safe_div", encoded, a, b)
		}
	}
	result := makeCase("result", "13_result_error.nanz")
	for i, f := range []string{"test_add_ok_tag", "test_add_ok_val", "test_add_overflow_tag", "test_add_overflow_code", "test_is_ok_pass", "test_is_ok_fail"} {
		add(&result, f, []int64{0, 30, 1, 1, 1, 0}[i])
	}
	for _, a := range []int64{0, 1, 2, 3, 15, 127, 128, 254, 255} {
		for _, b := range []int64{0, 1, 2, 3, 15, 127, 128, 254, 255} {
			encoded := a + b // Ok: tag 0, byte sum
			if encoded > 255 {
				encoded = 257
			} // Err: tag 1, error code 1
			add(&result, "safe_add", encoded, a, b)
		}
	}
	recursion := makeCase("recursion", "18_tail_recursion.nanz")
	for _, n := range []int64{0, 1, 2, 3, 5, 7, 10, 16, 32, 127, 255} {
		a, b := uint8(0), uint8(1)
		fact := uint8(1)
		for j := int64(0); j < n; j++ {
			a, b = b, a+b
		}
		for j := int64(2); j <= n; j++ {
			fact *= uint8(j)
		}
		add(&recursion, "fib", int64(a), n)
		add(&recursion, "fact", int64(fact), n)
		add(&recursion, "sum_to", int64(uint8(n*(n+1)/2)), n)
		add(&recursion, "pow2", int64(uint8(uint64(1)<<uint(n))), n)
	}
	return []profileProgramCase{arena, sm, option, result, recursion}
}

func TestProfileCompleteSourcePrograms(t *testing.T) {
	for _, c := range profileProgramCases() {
		t.Run(c.name, func(t *testing.T) { profileProgramRun(t, c) })
	}
}

func profileProgramCheck(call hir.ProfileInvocation, values []mir2.Value, want int64) error {
	if len(values) != 1 || values[0].I != want {
		return fmt.Errorf("%s: got %v, independent oracle wants %d", call.Function, values, want)
	}
	return nil
}

func profileProgramRun(t *testing.T, c profileProgramCase) {
	t.Helper()
	root := os.Getenv("MINZ_PGO_PROGRAM_ARTIFACT_DIR")
	if root == "" {
		root = t.TempDir()
	}
	dir := filepath.Join(root, c.name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, v any) {
		t.Helper()
		var data []byte
		switch value := v.(type) {
		case []byte:
			data = value
		case string:
			data = []byte(value)
		default:
			var err error
			data, err = json.MarshalIndent(v, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	sourcePath := filepath.Join("..", "..", "..", "examples", "nanz", c.source)
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	write("source.nanz", source)
	write("workload.json", c.workload)
	write("oracle.json", map[string]any{"results": c.expected, "snapshots": c.snapshots, "globals": c.snapshots[len(c.snapshots)-1].Globals, "resources": map[string][]byte{}})
	factory := func() (*hir.Module, error) { return nanz.Parse(string(source), sourcePath) }
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	write("source-manifest.json", m.SourceUnits)
	if m.SourceUnits[sourcePath] != fmt.Sprintf("%x", sha256.Sum256(source)) {
		t.Fatal("full unchanged source digest missing")
	}
	raw, err := hir.RunLoopProfile(m, c.workload)
	write("profile-run.json", raw)
	if err != nil {
		write("failure.txt", err.Error())
		t.Fatal(err)
	}
	write("profile.json", raw.Profile)
	checkRaw := func(run *hir.ProfileRun) {
		t.Helper()
		if len(run.Results) != len(c.expected) {
			t.Fatal("incomplete raw results")
		}
		for i, values := range run.Results {
			if err := profileProgramCheck(c.workload.Calls[i], values, c.expected[i]); err != nil {
				t.Fatalf("call %d: %v", i, err)
			}
		}
		if !reflect.DeepEqual(run.Snapshots, c.snapshots) || !reflect.DeepEqual(run.Globals, c.snapshots[len(c.snapshots)-1].Globals) || len(run.Resources) != 0 {
			t.Fatal("independent memory oracle mismatch")
		}
	}
	checkRaw(raw)
	if raw.Profile.CompletedCalls != uint64(len(c.expected)) || raw.Profile.FailedCalls != 0 {
		t.Fatal("profile incomplete")
	}
	profile, err := json.Marshal(raw.Profile)
	if err != nil {
		t.Fatal(err)
	}
	options := Options{AssertMode: "none", UseHIRGrace: true}
	measured, measureErr := MeasureGraceVariantSets(factory, profile, c.workload, options, 4)
	// Export and execute both structural policies even if the production selector
	// returns nil evidence on a failure. Do not change or shrink the workload.
	failures := []string{}
	costs := []map[string]any{}
	for i, enabled := range []bool{false, true} {
		input, err := factory()
		if err != nil {
			t.Fatal(err)
		}
		input.GraceRulePolicy = map[string]bool{"hir-constant-if": enabled, "hir-invariant-base": enabled}
		input.PointerWalkPolicy = map[string]bool{}
		steps, err := CompileHIRSteps(input, options)
		if err != nil {
			t.Fatal(err)
		}
		prefix := fmt.Sprintf("candidate-%d", i)
		write(prefix+".a80", steps.Assembly)
		write(prefix+".mir", steps.MIR2Opt)
		write(prefix+".hir", input.Dump())
		assembled, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if err != nil {
			t.Fatal(err)
		}
		if len(assembled.Errors) > 0 {
			t.Fatal(assembled.Errors)
		}
		write(prefix+".bin", assembled.Binary)
		write(prefix+"-symbols.json", assembled.Symbols)
		target, targetErr := RunZ80ProfileWorkload(steps, m, c.workload, *raw)
		write(prefix+"-results.json", target)
		cost := map[string]any{"candidate": i, "common_grace_rules": enabled, "module_bytes": len(assembled.Binary), "binary_sha256": fmt.Sprintf("%x", sha256.Sum256(assembled.Binary))}
		if targetErr != nil {
			failures = append(failures, fmt.Sprintf("candidate %d: %v", i, targetErr))
			cost["error"] = targetErr.Error()
		} else {
			if len(target.Invocations) != len(c.expected) {
				t.Fatal("incomplete target")
			}
			var total int64
			for j, inv := range target.Invocations {
				if err := profileProgramCheck(c.workload.Calls[j], inv.Results, c.expected[j]); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(inv.Snapshot, c.snapshots[j]) {
					t.Fatalf("target snapshot %d mismatch", j)
				}
				if inv.TStates <= 0 {
					t.Fatal("missing timing")
				}
				total += inv.TStates
			}
			if !reflect.DeepEqual(target.Globals, raw.Globals) || !reflect.DeepEqual(target.Resources, raw.Resources) {
				t.Fatal("target final memory mismatch")
			}
			cost["total_t_states"] = total
			cost["executed_steps"] = target.ExecutedSteps
			if measured != nil {
				candidate := measured.Evidence.Candidates[i]
				checkRaw(candidate.RawRun)
				if !reflect.DeepEqual(candidate.Run, target) || !bytes.Equal(candidate.Binary, assembled.Binary) || candidate.TotalTStates != total {
					t.Fatal("measurement reproduction mismatch")
				}
			}
		}
		costs = append(costs, cost)
	}
	write("costs.json", costs)
	if measureErr != nil {
		write("evidence.json", map[string]any{"status": "rejected", "error": measureErr.Error(), "direct_failures": failures, "options": options})
		write("results.json", map[string]any{"status": "rejected", "error": measureErr.Error(), "direct_failures": failures, "raw_completed_calls": len(c.expected), "raw_verified_snapshots": len(c.snapshots), "max_steps": c.workload.MaxSteps, "max_memory": c.workload.MaxMemory})
		t.Fatalf("measurement: %v; direct: %v; artifacts %s", measureErr, failures, dir)
	}
	if len(failures) > 0 {
		t.Fatal(failures)
	}
	e := measured.Evidence
	write("evidence.json", e)
	checkRaw(e.Oracle)
	if len(e.EligibleSites) != 0 || len(e.Candidates) != 2 || e.Candidates[0].CommonGraceRules || !e.Candidates[1].CommonGraceRules {
		t.Fatal("expected baseline/common comparison")
	}
	best := 0
	if e.Candidates[1].TotalTStates < e.Candidates[0].TotalTStates {
		best = 1
	}
	if e.Selected != best {
		t.Fatal("selection not strict speed improvement")
	}
	if !bytes.Equal(measured.Binary, e.Candidates[best].Binary) {
		t.Fatal("selected binary differs from measured winner")
	}
	selected, err := RunZ80ProfileWorkload(measured.Steps, m, c.workload, *raw)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(selected, e.Candidates[best].Run) {
		t.Fatal("selected replay differs from measured winner")
	}
	write("selected-results.json", selected)
	write("selected.a80", measured.Steps.Assembly)
	write("selected.bin", measured.Binary)
	write("results.json", map[string]any{"status": "verified", "calls": len(c.expected), "selected": e.Selected, "max_steps": c.workload.MaxSteps, "max_memory": c.workload.MaxMemory})
	t.Logf("%d calls: baseline %dT/%dB common %dT/%dB selected %d; artifacts %s", len(c.expected), e.Candidates[0].TotalTStates, e.Candidates[0].ModuleBytes, e.Candidates[1].TotalTStates, e.Candidates[1].ModuleBytes, e.Selected, dir)
}
