package mir2

import "slices"

// cpmDefs lists every definition of each register of one function. After
// pre-allocation coalescing a register can be an instruction result and a
// block parameter at once, so facts about "the" definition hold only when
// that definition is the register's sole one.
type cpmDefs struct {
	insts    map[Reg][]*Inst      // instructions with Dst == r
	params   map[Reg][]BlockParam // block parameters that are r
	feeders  map[Reg][]Reg        // other registers passed to those parameters
	implicit map[Reg]bool         // a definition without an instruction or edge argument
}

// Implicit definitions are function parameters, extra call results, inline-asm
// outputs, the DJNZ counter position, and parameters on an edge whose argument
// count does not match the target's parameters.
func cpmFuncDefs(f *Func) *cpmDefs {
	d := &cpmDefs{insts: map[Reg][]*Inst{}, params: map[Reg][]BlockParam{}, feeders: map[Reg][]Reg{}, implicit: map[Reg]bool{}}
	for _, p := range f.Contract.Params {
		d.implicit[p.Reg] = true
	}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			d.params[p.Dst] = append(d.params[p.Dst], p)
		}
		for _, i := range b.Insts {
			if i.Dst != NoReg {
				d.insts[i.Dst] = append(d.insts[i.Dst], i)
			}
			for _, r := range i.ExtraRets {
				d.implicit[r] = true
			}
			if i.Asm != nil {
				for _, r := range i.Asm.Outs {
					d.implicit[r] = true
				}
			}
		}
	}
	for _, b := range f.Blocks {
		if b.Term == nil {
			continue
		}
		b.Term.ForEachEdge(func(target string, args []Reg, offset int) {
			for _, dst := range f.Blocks {
				if dst.Label != target {
					continue
				}
				exact := offset >= 0 && offset <= len(dst.Params) && len(args) == len(dst.Params)-offset
				for n, p := range dst.Params {
					if !exact || n < offset || n-offset >= len(args) {
						d.implicit[p.Dst] = true
						continue
					}
					if a := args[n-offset]; a != p.Dst && !slices.Contains(d.feeders[p.Dst], a) {
						d.feeders[p.Dst] = append(d.feeders[p.Dst], a)
					}
				}
			}
		})
	}
	return d
}

// sole returns r's defining instruction when it is r's only definition.
func (d *cpmDefs) sole(r Reg) *Inst {
	if len(d.insts[r]) != 1 || len(d.params[r]) != 0 || d.implicit[r] {
		return nil
	}
	return d.insts[r][0]
}
