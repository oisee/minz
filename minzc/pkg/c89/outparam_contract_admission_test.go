package c89_test

import (
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"testing"
)

func TestC89OutParamContractRefusals(t *testing.T) {
	prefix := `typedef struct {unsigned char a,b;} Pair;`
	cases := map[string]string{
		"global-observer":        `Pair g;unsigned char seen;void fill(unsigned char x,Pair*out){out->a=x;seen=g.a;out->b=x+1;}unsigned char execute(void){g.a=99;fill(7,&g);return seen;}`,
		"alias-input":            `void fill(Pair*in,Pair*out){out->a=7;out->b=in->a;}unsigned char execute(void){Pair p={99,0};fill(&p,&p);return p.b;}`,
		"output-read":            `void fill(unsigned char x,Pair*out){out->a=x;out->b=out->a+1;}unsigned char execute(void){Pair p;fill(7,&p);return p.b;}`,
		"escape-output":          `Pair*saved;void fill(unsigned char x,Pair*out){saved=out;out->a=x;out->b=x+1;}unsigned char execute(void){Pair p;fill(7,&p);return saved->a;}`,
		"partial-return":         `void fill(unsigned char x,Pair*out){out->a=x;if(x) return;out->b=7;}unsigned char execute(void){Pair p={99,42};fill(1,&p);return p.b;}`,
		"partial-branch":         `void fill(unsigned char x,Pair*out){if(x)out->a=x;else out->b=x;}unsigned char execute(void){Pair p={99,42};fill(1,&p);return p.b;}`,
		"caller-escape":          `Pair*saved;void fill(unsigned char x,Pair*out){out->a=x;out->b=x+1;}unsigned char execute(void){Pair p;saved=&p;fill(7,&p);return saved->b;}`,
		"callee-call":            `unsigned char observe(void){return 7;}void fill(unsigned char x,Pair*out){out->a=x;out->b=observe();}unsigned char execute(void){Pair p;fill(7,&p);return p.b;}`,
		"unknown-second-call":    `void fill(unsigned char x,Pair*out){out->a=x;out->b=x+1;}void another(Pair*p){fill(7,p);}unsigned char execute(void){Pair p;fill(3,&p);another(&p);return p.b;}`,
		"volatile-object":        `void fill(unsigned char x,volatile Pair*out){out->a=x;out->b=x+1;}unsigned char execute(void){volatile Pair p;fill(7,&p);return p.b;}`,
		"unproven-pointer-read":  `unsigned char *probe,seen;void fill(unsigned char x,Pair*out){out->a=x;seen=*probe;out->b=x+1;}unsigned char execute(void){Pair p;probe=(unsigned char*)0xeff0;fill(7,&p);return seen;}`,
		"unproven-pointer-write": `unsigned char *probe;void fill(unsigned char x,Pair*out){out->a=x;*probe=9;out->b=x+1;}unsigned char execute(void){Pair p;probe=(unsigned char*)0xeff0;fill(7,&p);return p.a;}`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			m, e := c89.CompileWithOpts(prefix+src, name+".c", c89.CompileOpts{NoLibc: true, DeferAggregateABI: true})
			if e != nil {
				t.Fatal(e)
			}
			f := m.FuncByName("fill")
			n := len(f.Params)
			hir.OptimizeCAggregateABI(m)
			if len(f.Params) != n || len(f.RetTys) != 0 {
				t.Fatalf("unsafe out-param admission: %+v", f)
			}
		})
	}
}

func TestC89OutParamContractWithExtern(t *testing.T) {
	src := `typedef struct {unsigned char a,b;} Pair;void fill(unsigned char x,Pair*out){out->a=x;out->b=x+1;}unsigned char execute(void){Pair p;fill(7,&p);return p.b;}`
	m, e := c89.CompileWithOpts(src, "extern-neighbor.c", c89.CompileOpts{NoLibc: true, DeferAggregateABI: true})
	if e != nil {
		t.Fatal(e)
	}
	m.Funcs = append(m.Funcs, &hir.Func{Name: "unrelated_extern", IsExtern: true})
	hir.OptimizeCAggregateABI(m)
	if len(m.FuncByName("fill").RetTys) != 2 {
		t.Fatal("unrelated extern blocks closed output contract")
	}
}
