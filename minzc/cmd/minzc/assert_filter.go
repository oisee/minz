package main

import "github.com/minz/minzc/pkg/hir"

func filterJudgeAsserts(as []hir.Assert, selected map[int]bool, selectLines bool, controlLine int, control bool) []hir.Assert {
	var out []hir.Assert
	for _, a := range as {
		if selectLines && !selected[a.Line] {
			continue
		}
		if control && a.Line == controlLine {
			if len(a.ExpectedMulti) > 0 {
				a.ExpectedMulti = append([]int64(nil), a.ExpectedMulti...)
				a.ExpectedMulti[len(a.ExpectedMulti)-1] ^= 1 << 32
			} else {
				a.Expected ^= 1 << 32
			}
		}
		out = append(out, a)
	}
	return out
}
