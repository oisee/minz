package hir_test

import (
	"crypto/sha256"
	"fmt"
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
	"strings"
	"testing"
)

func TestGotoC89SourceVMZ80(t *testing.T) {
	const src = `unsigned short skip(void){int x=1;goto done;x=99;done:return x;}
unsigned short outward(void){int x=7;int i=0;again:{i=i+1;int x=99;if(i<2)goto again;}return x;}
unsigned short forward(void){int x=7;{x=9;int x=99;goto done;}x=88;done:return x;}
unsigned short static_shadow(void){static int x=7;{int x=99;x=x+1;}return x;}
unsigned short dead_both(void){int x=7;goto b;if(0){a:x=9;goto out;}else{b:x=11;goto out;}out:return x;}
unsigned short loop_entry(void){goto a;for(int i=0;i<3;i++){a:return 7;}return 0;}
unsigned short dead_switch(void){int x=7;goto a;switch(x){case 7:a:x=9;break;default:x=3;}return x;}
unsigned short branch_assign(void){goto a;int x=99;a:if(1)x=7;else x=8;return x;}
unsigned short return_assign(void){goto a;int x=99;a:if(0){return 7;}else{x=2;}return x;}
unsigned short assigned_before_shadow(void){goto a;int x=99;a:if(1){x=2;int x=3;x=4;}else{x=9;}return x;}
// assert static_shadow() == 7
// assert skip() == 1
// assert outward() == 7
// assert forward() == 9
// assert dead_both() == 11
// assert loop_entry() == 7
// assert dead_switch() == 9
// assert branch_assign() == 7
// assert return_assign() == 2
// assert assigned_before_shadow() == 2
`
	m, e := c89.Compile(src, "goto-source.c")
	if e != nil {
		t.Fatal(e)
	}
	m.SourceUnitsComplete = true
	m.SourceUnits = map[string]string{"goto-source.c": fmt.Sprintf("%x", sha256.Sum256([]byte(src)))}
	// C89 does not yet emit origin metadata. The sole structured loop is
	// explicitly tied to its exact source token for this profile harness.
	m.SourceLocations = map[hir.Stmt]hir.SourceLocation{}
	line := 1
	column := 1
	token := strings.Index(src, "for(int")
	if token < 0 {
		t.Fatal("missing fixture for token")
	}
	loops := 0
	for i := 0; i < token; i++ {
		if src[i] == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}
	var origins func(*hir.Block)
	origins = func(b *hir.Block) {
		if b == nil {
			return
		}
		for _, s := range b.Body {
			switch x := s.(type) {
			case *hir.WhileStmt:
				loops++
				m.SourceLocations[s] = hir.SourceLocation{ByteOffset: token, Unit: "goto-source.c", SourceSHA256: m.SourceUnits["goto-source.c"], Line: line, Column: column}
				origins(x.Body)
			case *hir.Block:
				origins(x)
			case *hir.IfStmt:
				origins(x.Then)
				origins(x.Else)
			}
		}
	}
	for _, f := range m.Funcs {
		if f.Name == "loop_entry" {
			origins(f.Body)
		}
	}
	if loops != 1 {
		t.Fatalf("expected sole loop_entry loop, got %d", loops)
	}
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{}, Globals: map[string][]byte{}, Calls: []hir.ProfileInvocation{{Function: "skip"}, {Function: "outward"}, {Function: "forward"}, {Function: "static_shadow"}, {Function: "dead_both"}, {Function: "loop_entry"}, {Function: "dead_switch"}, {Function: "branch_assign"}, {Function: "return_assign"}, {Function: "assigned_before_shadow"}}}
	raw, e := hir.RunLoopProfile(m, w)
	if e != nil {
		t.Fatal(e)
	}
	want := []int64{1, 7, 9, 7, 11, 7, 9, 7, 2, 2}
	for i, n := range want {
		if raw.Results[i][0].I != n {
			t.Fatalf("Craw%dgot%v want%d", i, raw.Results[i], n)
		}
	}
	steps, e := pipeline.CompileHIRSteps(m, pipeline.Options{AssertMode: "all", UseHIRGrace: true})
	if e != nil {
		t.Fatal(e)
	}
	run, e := pipeline.RunZ80ProfileWorkload(steps, m, w, *raw)
	if e != nil {
		t.Fatal(e)
	}
	for i, n := range want {
		if run.Invocations[i].Results[0].I != n {
			t.Fatalf("Ctarget%dgot%v want%d", i, run.Invocations[i].Results, n)
		}
	}
}

func TestGotoC89SkippedInitializationBranchRejected(t *testing.T) {
	src := `unsigned short f(int flag){goto a;int x=99;a:if(flag){goto b;x=1;}else{x=2;}b:return x;}`
	m, err := c89.Compile(src, "goto-skipped.c")
	if err == nil {
		err = hir.Verify(m)
	}
	if err == nil || !strings.Contains(err.Error(), "skips lexical binding x") {
		t.Fatalf("expected skipped binding refusal, got %v", err)
	}
}

func TestGotoC89ShadowDoesNotInitializeOuter(t *testing.T) {
	for _, body := range []string{`if(c){int x=1;x=2;}else{int x=3;x=4;}`, `if(c){{int x=3;x=4;}}else{x=2;}`, `if(c){int x=1;x=2;goto b;}else{x=3;goto b;}`} {
		m, err := c89.Compile("unsigned short f(int c){goto a;int x=99;a:"+body+"b:return x;}", "goto-shadow.c")
		if err == nil {
			err = hir.Verify(m)
		}
		if err == nil || !strings.Contains(err.Error(), "skips lexical binding x") {
			t.Fatalf("expected outer initialization refusal, got %v", err)
		}
	}
}
