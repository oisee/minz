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
	"strings"
	"testing"
)

func TestZ80ProfileWorkloadDeadExternAndFetchDenial(t *testing.T) {
	const text = `@extern fun disk_read(p:^u8,n:u16)->u8
fun dead()->u8 {return disk_read(0,1)}
fun entry()->u8 {return 173}
`
	w := workloadSpec(hir.ProfileInvocation{Function: "entry"})
	steps, source, oracle := workloadFixture(t, text, w)
	out, e := RunZ80ProfileWorkload(steps, source, w, oracle)
	if e != nil || len(out.Invocations) != 1 || out.Invocations[0].Results[0].I != 173 {
		t.Fatal(out, e, steps.Assembly)
	}
	if !strings.Contains(steps.Assembly, "disk_read:") {
		t.Fatal("extern declaration trimmed")
	}
	bad := steps
	bad.Assembly = strings.Replace(steps.Assembly, "entry:\n", "entry:\n JP dead\n", 1)
	if bad.Assembly == steps.Assembly {
		t.Fatal("missing entry symbol")
	}
	denied, e := RunZ80ProfileWorkload(bad, source, w, oracle)
	if denied != nil || e == nil || !strings.Contains(e.Error(), "execution outside instruction bounds") {
		t.Fatal("dead native fetch accepted", denied, e)
	}
}
func TestProfileWorkloadPrivateNameCollision(t *testing.T) {
	const text = `@extern fun __minz_loop_profile_0_entry()->u8
fun entry()->u8 {var i:u8=0
 while i<4 {i=i+1}
 return i}`
	m, e := nanz.Parse(text, "private-collision.nanz")
	if e != nil {
		t.Fatal(e)
	}
	w := workloadSpec(hir.ProfileInvocation{Function: "entry"})
	r, e := hir.RunLoopProfile(m, w)
	if e != nil || r.Results[0][0].I != 4 || len(r.Profile.Counts) != 1 || r.Profile.Counts[0].Entries != 1 || r.Profile.Counts[0].BodyEntries != 4 {
		t.Fatal(r, e)
	}
	live := strings.Replace(text, "return i", "return __minz_loop_profile_0_entry()", 1)
	m, e = nanz.Parse(live, "private-collision.nanz")
	if e != nil {
		t.Fatal(e)
	}
	if r, e := hir.RunLoopProfile(m, w); r != nil || e == nil {
		t.Fatal("user prefix gained host authority", r, e)
	}
}

func TestFATPackedControlledWholeSourceAdmission(t *testing.T) {
	const path = "../../../stdlib/fs/fat12.nanz"
	src, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	for _, mode := range []Options{{AssertMode: "none"}, {AssertMode: "none", UseGrace: true}} {
		fresh := func() *hir.Module {
			m, e := nanz.Parse(string(src), path)
			if e != nil {
				t.Fatal(e)
			}
			return m
		}
		source := fresh()
		steps, e := CompileHIRSteps(fresh(), mode)
		if e != nil {
			t.Fatal(e)
		}
		if dir := os.Getenv("MINZ_PROFILE_DEAD_EXTERN_ARTIFACTS"); dir != "" {
			name := fmt.Sprintf("grace-%v", mode.UseGrace)
			if e := os.MkdirAll(dir, 0755); e != nil {
				t.Fatal(e)
			}
			program, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
			if e != nil || len(program.Errors) != 0 {
				t.Fatal(e)
			}
			for file, data := range map[string][]byte{name + ".a80": []byte(steps.Assembly), name + ".bin": program.Binary, "source.nanz": src} {
				if e := os.WriteFile(filepath.Join(dir, file), data, 0644); e != nil {
					t.Fatal(e)
				}
			}
		}
		for _, part := range []string{"training", "heldout"} {
			b, e := os.ReadFile("../../../reports/fat-packed-prospective/" + part + "-controlled.json")
			if e != nil {
				t.Fatal(e)
			}
			var w hir.ProfileWorkload
			if e = json.Unmarshal(b, &w); e != nil {
				t.Fatal(e)
			}
			oracle, e := hir.RunLoopProfile(source, w)
			if e != nil {
				t.Fatal(e)
			}
			fatPackedCheckIndependent(t, w, oracle.Results, oracle.Snapshots)
			run, e := RunZ80ProfileWorkload(steps, source, w, *oracle)
			if e != nil {
				t.Fatal(e)
			}
			rows := [][]mir2.Value{}
			snap := []hir.ProfileMemorySnapshot{}
			for _, call := range run.Invocations {
				rows = append(rows, call.Results)
				snap = append(snap, call.Snapshot)
			}
			fatPackedCheckIndependent(t, w, rows, snap)
			t.Logf("Grace%v %s %dcalls %dT SHA%s", mode.UseGrace, part, len(rows), run.TotalTStates, run.BinarySHA256)
		}
	}
}
