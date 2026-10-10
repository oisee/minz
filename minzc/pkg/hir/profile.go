package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
)

// LoopCounts counts statement entries (including zero-trip loops) and body
// entries. An iteration interrupted inside its body is still a body entry.
type LoopCounts struct {
	SiteID      string `json:"site_id"`
	Entries     uint64 `json:"entries"`
	BodyEntries uint64 `json:"body_entries"`
}

type LoopProfile struct {
	Compatibility  *ProfileCompatibility `json:"compatibility"`
	Schema         string                `json:"schema"`
	Stage          string                `json:"stage"`
	Sites          *LoopSiteMap          `json:"sites"`
	Counts         []LoopCounts          `json:"counts"`
	CompletedCalls uint64                `json:"completed_calls"`
	FailedCalls    uint64                `json:"failed_calls"`
}

// LoopProfiler executes a private instrumented copy of input HIR on MIR2 VM.
// Its MIR must not be optimised or emitted to a target. Counts are observations,
// not legality evidence or a reusable optimisation profile.
type LoopProfiler struct {
	VM                *mir2.VM
	compatibility     *ProfileCompatibility
	profileHooks      map[string]mir2.HostFunc
	sites             *LoopSiteMap
	counts            []LoopCounts
	completed, failed uint64
}

// NewLoopProfiler leaves the caller's HIR untouched. Missing source origins
// remain unmapped. Hosts/heap and gas limit can be configured through VM.
func NewLoopProfiler(m *Module) (*LoopProfiler, error) {
	sites, err := BuildLoopSiteMap(m)
	if err != nil {
		return nil, err
	}
	p := &LoopProfiler{sites: sites, counts: make([]LoopCounts, len(sites.Sites))}
	byPath := map[string]map[string]int{}
	for i, s := range sites.Sites {
		p.counts[i].SiteID = s.ID
		if byPath[s.Function] == nil {
			byPath[s.Function] = map[string]int{}
		}
		byPath[s.Function][s.HIRPath] = i
	}
	// Choose a namespace absent from both declarations and source calls.
	used := map[string]bool{}
	for _, f := range m.Funcs {
		used[f.Name] = true
		walkGraceBlocks(f.Body, "", func(_ string, b *Block) {
			for _, s := range b.Body {
				visitStmtExprs(s, func(e Expr) {
					if c, ok := e.(*CallExpr); ok {
						used[c.Fn] = true
					}
				})
			}
		})
	}
	prefix := "__minz_loop_profile_"
	for n := 0; ; n++ {
		collision := false
		for i := range sites.Sites {
			for _, event := range []string{"entry", "body"} {
				if used[fmt.Sprintf("%s%d_%s", prefix, i, event)] {
					collision = true
				}
			}
		}
		if !collision {
			break
		}
		prefix = fmt.Sprintf("__minz_loop_profile_%d_", n)
	}
	copyModule := *m
	copyModule.Funcs = nil
	hooks := map[string]mir2.HostFunc{}
	eventStmt := func(i int, event string) Stmt {
		name := fmt.Sprintf("%s%d_%s", prefix, i, event)
		return &ExprStmt{Expr: &CallExpr{Fn: name, Ty: mir2.TyVoid}}
	}
	var cloneBlock func(*Block, string, map[string]int) *Block
	cloneBlock = func(b *Block, path string, paths map[string]int) *Block {
		if b == nil {
			return nil
		}
		out := &Block{}
		for j, s := range b.Body {
			sp := fmt.Sprintf("%s/s%d", path, j)
			index, mapped := paths[sp]
			var loopBody *Block
			switch st := s.(type) {
			case *Block:
				s = cloneBlock(st, sp, paths)
			case *IfStmt:
				c := *st
				c.Then = cloneBlock(st.Then, sp+"/then", paths)
				c.Else = cloneBlock(st.Else, sp+"/else", paths)
				s = &c
			case *WhileStmt:
				c := *st
				c.Body = cloneBlock(st.Body, sp+"/while", paths)
				loopBody = c.Body
				s = &c
			case *ForRangeStmt:
				c := *st
				c.Body = cloneBlock(st.Body, sp+"/range", paths)
				loopBody = c.Body
				s = &c
			case *ForEachStmt:
				c := *st
				c.Body = cloneBlock(st.Body, sp+"/each", paths)
				loopBody = c.Body
				s = &c
			case *SwitchStmt:
				c := *st
				c.Cases = make([]*SwitchCase, len(st.Cases))
				for k := range c.Cases {
					cc := *st.Cases[k]
					cc.Body = cloneBlock(st.Cases[k].Body, fmt.Sprintf("%s/case%d", sp, k), paths)
					c.Cases[k] = &cc
				}
				c.Default = cloneBlock(st.Default, sp+"/default", paths)
				s = &c
			}
			if mapped && loopBody != nil {
				out.Body = append(out.Body, eventStmt(index, "entry"))
				loopBody.Body = append([]Stmt{eventStmt(index, "body")}, loopBody.Body...)
			}
			out.Body = append(out.Body, s)
		}
		return out
	}
	for _, f := range m.Funcs {
		c := *f
		c.Body = cloneBlock(f.Body, "body", byPath[f.Name])
		copyModule.Funcs = append(copyModule.Funcs, &c)
	}
	for i := range sites.Sites {
		for _, event := range []string{"entry", "body"} {
			name := fmt.Sprintf("%s%d_%s", prefix, i, event)
			copyModule.Funcs = append(copyModule.Funcs, &Func{Name: name, IsExtern: true, RetTy: mir2.TyVoid})
			hooks[name] = func(_ []mir2.Value) ([]mir2.Value, error) {
				if event == "entry" {
					p.counts[i].Entries++
				} else {
					p.counts[i].BodyEntries++
				}
				return nil, nil
			}
		}
	}
	if err := Verify(&copyModule); err != nil {
		return nil, err
	}
	lowered := LowerModule(&copyModule)
	if err := mir2.Verify(lowered); err != nil {
		return nil, err
	}
	p.profileHooks = hooks
	p.VM = mir2.NewVM(lowered)
	for name, hook := range hooks {
		p.VM.Hosts[name] = hook
	}
	return p, nil
}

// NewLoopProfilerWithContext attaches a conservative compatibility key. The
// harness must retain the declared hosts/workload/VM semantics during collection.
func NewLoopProfilerWithContext(m *Module, c ProfileContext) (*LoopProfiler, error) {
	key, err := BuildProfileCompatibility(m, c)
	if err != nil {
		return nil, err
	}
	p, err := NewLoopProfiler(m)
	if err != nil {
		return nil, err
	}
	p.compatibility = key
	return p, nil
}

// Call accumulates events across calls, retaining partial counts on errors.
func (p *LoopProfiler) Call(name string, args []mir2.Value) ([]mir2.Value, error) {
	result, err := p.VM.Call(name, args)
	if err != nil {
		p.failed++
	} else {
		p.completed++
	}
	return result, err
}

// Snapshot returns an independent copy, including source hashes. It deliberately
// snapshots without an explicit context are diagnostic-only and refuse loading.
func (p *LoopProfiler) Snapshot() LoopProfile {
	sites := *p.sites
	sites.Sites = append([]LoopSite{}, p.sites.Sites...)
	var key *ProfileCompatibility
	if p.compatibility != nil {
		c := *p.compatibility
		c.Context.Dependencies = map[string]string{}
		for n, h := range p.compatibility.Context.Dependencies {
			c.Context.Dependencies[n] = h
		}
		key = &c
	}
	return LoopProfile{Compatibility: key, Schema: "minz.input-hir-loop-observations/v2", Stage: "unoptimised-mir2-vm", Sites: &sites, Counts: append([]LoopCounts{}, p.counts...), CompletedCalls: p.completed, FailedCalls: p.failed}
}
