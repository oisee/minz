package pipeline

import (
	"crypto/sha256"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func normalDiamondSource(t *testing.T) ([]byte, func() (*hir.Module, error)) {
	t.Helper()
	source, err := os.ReadFile("../../../examples/nanz/self_lanz_parser.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(source)) != "0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526" {
		t.Fatal("source identity")
	}
	return source, func() (*hir.Module, error) {
		m, e := nanz.Parse(string(source), "../examples/nanz/self_lanz_parser.nanz")
		if e == nil {
			m.Target = hir.TargetCPM
		}
		return m, e
	}
}

func assertNormalDiamondDelta(t *testing.T, m *mir2.Module, apply bool) (string, string, int) {
	t.Helper()
	before := m.Dump()
	old := map[*mir2.Inst]mir2.Inst{}
	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				old[i] = *i
			}
		}
	}
	count := 0
	if apply {
		var err error
		count, err = mir2.ReuseNormalGlobalDiamondLoads(m)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("actual source edits=%d want1", count)
		}
	}
	edits := 0
	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				prior := old[i]
				if reflect.DeepEqual(prior, *i) {
					continue
				}
				edits++
				if f.Name != "cur" || prior.Op != mir2.OpLoad || i.Op != mir2.OpMove || i.Ty != mir2.TyU16 || i.Dst != prior.Dst {
					t.Fatalf("unexpected actual edit %s %v→%v", f.Name, prior.Op, i.Op)
				}
				priorIsPos := false
				for _, address := range b.Insts {
					if address.Op == mir2.OpAddrOf && address.Dst == prior.Src[0] && address.Sym == "pos" {
						priorIsPos = true
					}
				}
				if !priorIsPos {
					t.Fatal("replaced load was not direct pos")
				}
				var expected mir2.Reg
				for _, entryInst := range f.Entry().Insts {
					if entryInst.Op == mir2.OpLoad {
						for _, address := range f.Entry().Insts {
							if address.Op == mir2.OpAddrOf && address.Sym == "pos" && address.Dst == entryInst.Src[0] {
								expected = entryInst.Dst
							}
						}
					}
				}
				wantMove := mir2.Inst{Op: mir2.OpMove, Dst: prior.Dst, Src: [2]mir2.Reg{expected, mir2.NoReg}, Ty: prior.Ty, Cls: prior.Cls, ClsHard: prior.ClsHard, SrcLine: prior.SrcLine, SrcFile: prior.SrcFile}
				if !reflect.DeepEqual(*i, wantMove) {
					t.Fatal("unexpected complete replacement metadata")
				}
				if expected == mir2.NoReg || i.Src[0] != expected {
					t.Fatal("move does not reuse dominating pos")
				}
			}
		}
	}
	if edits != count {
		t.Fatalf("delta edits%d helper%d", edits, count)
	}
	if err := mir2.Verify(m); err != nil {
		t.Fatal(err)
	}
	return before, m.Dump(), count
}

func TestNormalDiamondActualSourceStatic(t *testing.T) {
	_, parse := normalDiamondSource(t)
	m, e := parse()
	if e != nil {
		t.Fatal(e)
	}
	raw := hir.LowerModule(m)
	before, after, count := assertNormalDiamondDelta(t, raw, true)
	t.Logf("actualcur edits%d beforeSHA%x afterSHA%x", count, sha256.Sum256([]byte(before)), sha256.Sum256([]byte(after)))
	if out := os.Getenv("MINZ_NORMAL_DIAMOND_STATIC_ARTIFACTS"); out != "" {
		if e = os.MkdirAll(out, 0755); e != nil {
			t.Fatal(e)
		}
		for n, b := range map[string][]byte{"before.mir2": []byte(before), "after.mir2": []byte(after)} {
			if e = os.WriteFile(filepath.Join(out, n), b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
}
