package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"testing"
)

func TestNegativeLiteralContextVMZ80(t *testing.T) {
	src, e := os.ReadFile("../../../reports/nanz-negative-literals/source.nanz")
	if e != nil {
		t.Fatal(e)
	}
	frozen, e := os.ReadFile("../../../reports/nanz-negative-literals/freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	var spec map[string]any
	if e = json.Unmarshal(frozen, &spec); e != nil || fmt.Sprintf("%x", sha256.Sum256(src)) != spec["sha256"] {
		t.Fatal("sourcefreeze", e)
	}
	parse := func() *hir.Module {
		h, e := nanz.Parse(string(src), "negative-literals.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	cases := []struct {
		f    string
		args []int64
		want int64
	}{{"literal", nil, 65525}, {"boundary128", nil, 65408}, {"boundary129", nil, 65407}, {"boundary32768", nil, 32768}, {"explicit", nil, 65525}, {"assigned", nil, 65525}, {"assigned_u", nil, 65525}, {"argument", nil, 65525}, {"conditional", []int64{0}, 65407}, {"conditional", []int64{1}, 65525}, {"variable", []int64{11}, 245}, {"neg_variable", []int64{11}, 245}}
	for _, o := range []Options{{AssertMode: "z80"}, {UseHIRGrace: true, UseGrace: true, AssertMode: "z80"}, {UseHIRGrace: true, UseGrace: true, OptSize: true, AssertMode: "z80"}} {
		h := parse()
		for _, c := range cases {
			h.Asserts = append(h.Asserts, hir.Assert{FuncName: c.f, Args: c.args, Expected: c.want, Via: "z80"})
		}
		s, e := CompileHIRSteps(h, o)
		if e != nil {
			t.Fatal(e)
		}
		if dir := os.Getenv("MINZ_NEGATIVE_LITERAL_ARTIFACTS"); dir != "" {
			mode := "ordinary"
			if o.UseGrace {
				mode = "grace-speed"
			}
			if o.OptSize {
				mode = "grace-size"
			}
			p := filepath.Join(dir, mode)
			if err := os.MkdirAll(p, 0755); err != nil {
				t.Fatal(err)
			}
			for n, b := range map[string][]byte{"target.a80": []byte(s.Assembly), "raw.mir": []byte(hir.LowerModule(parse()).Dump()), "final.mir": []byte(s.MIR2Module.Dump())} {
				if err := os.WriteFile(filepath.Join(p, n), b, 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
		for _, m := range []*mir2.Module{hir.LowerModule(parse()), s.MIR2Module} {
			for _, c := range cases {
				args := []mir2.Value{}
				for _, v := range c.args {
					args = append(args, mir2.Value{I: v})
				}
				v, e := mir2.NewVM(m).Call(c.f, args)
				if e != nil || len(v) != 1 || v[0].I != c.want {
					t.Fatal(c, v, e)
				}
			}
		}
	}
}
