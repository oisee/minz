// mulopt.go — GPU-optimal constant multiplication table.
//
// Loads precomputed optimal multiply-by-constant sequences from
// z80-optimizer's mulopt8_clobber.json. Each entry: A × K → A
// with clobber annotation (which registers are destroyed).
//
// Two clobber classes:
//   B-preserving: {A, F, carry} — 14 solutions (powers of 2 + NEG-based)
//   B-clobbering: {A, B, F, carry} — 150 solutions (B used as temp)
package vir

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type MulOpt struct {
	K        int      `json:"k"`
	Ops      []string `json:"ops"`
	TStates  int      `json:"tstates"`
	Clobber  []string `json:"clobber"`
}

type MulOptTable struct {
	entries map[int]*MulOpt // K → optimal sequence
}

var (
	globalMulOpt     *MulOptTable
	globalMulOptOnce sync.Once
)

func GetMulOptTable() *MulOptTable {
	globalMulOptOnce.Do(func() {
		globalMulOpt = loadMulOptTable()
	})
	return globalMulOpt
}

func loadMulOptTable() *MulOptTable {
	t := &MulOptTable{entries: make(map[int]*MulOpt)}

	// Try multiple paths
	paths := []string{
		os.ExpandEnv("$HOME/dev/z80-optimizer/data/mulopt8_clobber.json"),
	}
	if p := os.Getenv("MULOPT8_PATH"); p != "" {
		paths = []string{p}
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var entries []MulOpt
		if err := json.Unmarshal(data, &entries); err != nil {
			fmt.Fprintf(os.Stderr, "[mulopt] parse error: %v\n", err)
			continue
		}
		for i := range entries {
			t.entries[entries[i].K] = &entries[i]
		}
		fmt.Fprintf(os.Stderr, "[mulopt] loaded %d mul8 entries from %s\n", len(entries), path)
		return t
	}

	return t
}

// Lookup returns the optimal multiply sequence for A × K → A.
// bPreserve=true returns only B-preserving solutions.
func (t *MulOptTable) Lookup(k int, bPreserve bool) *MulOpt {
	if t == nil {
		return nil
	}
	e, ok := t.entries[k]
	if !ok {
		return nil
	}
	if bPreserve {
		for _, c := range e.Clobber {
			if c == "B" {
				return nil // B is clobbered, caller needs B-preserving
			}
		}
	}
	return e
}

func (t *MulOptTable) Size() int {
	if t == nil {
		return 0
	}
	return len(t.entries)
}

// ── Widening / 16-bit multiply table ────────────────────────────────────────
// GPU-optimal HL×K→HL sequences from mulopt16_complete.json.
// For u8→u16 widening: prepend LD L,A; LD H,0 (11T).
// For u16×K: use directly.

type Mul16Opt struct {
	K       int      `json:"k"`
	Ops     []string `json:"ops"`
	Length  int      `json:"length"`
	TStates int      `json:"tstates"`
	Clobber []string `json:"clobber"`
}

type Mul16OptTable struct {
	entries map[int]*Mul16Opt
}

var (
	globalMul16Opt     *Mul16OptTable
	globalMul16OptOnce sync.Once
)

func GetMul16OptTable() *Mul16OptTable {
	globalMul16OptOnce.Do(func() {
		globalMul16Opt = loadMul16OptTable()
	})
	return globalMul16Opt
}

func loadMul16OptTable() *Mul16OptTable {
	t := &Mul16OptTable{entries: make(map[int]*Mul16Opt)}

	paths := []string{
		os.ExpandEnv("$HOME/dev/z80-optimizer/data/mulopt16_complete.json"),
	}
	if p := os.Getenv("MUL16_PATH"); p != "" {
		paths = []string{p}
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var entries []Mul16Opt
		if err := json.Unmarshal(data, &entries); err != nil {
			fmt.Fprintf(os.Stderr, "[mul16opt] parse error: %v\n", err)
			continue
		}
		for i := range entries {
			t.entries[entries[i].K] = &entries[i]
		}
		fmt.Fprintf(os.Stderr, "[mul16opt] loaded %d mul16 entries from %s\n", len(entries), path)
		return t
	}

	return t
}

// Lookup returns the optimal multiply sequence for HL × K → HL.
func (t *Mul16OptTable) Lookup(k int) *Mul16Opt {
	if t == nil {
		return nil
	}
	return t.entries[k]
}

func (t *Mul16OptTable) Size() int {
	if t == nil {
		return 0
	}
	return len(t.entries)
}

// ── u32 operations table ────────────────────────────────────────────────────
// GPU-verified optimal u32 (DEHL) operations from z80-optimizer.

type U32Op struct {
	Ops     []string `json:"ops"`
	Length  int      `json:"length"`
	Bytes   int      `json:"bytes"`
	TStates any      `json:"tstates"` // int or string ("32-40" for branch)
	Clobber []string `json:"clobbers"`
	Proven  bool     `json:"proven_optimal"`
	Notes   string   `json:"notes"`
}

type U32OpsTable struct {
	SHL32     *U32Op `json:"shl32"`
	SHR32     *U32Op `json:"shr32"`
	SAR32     *U32Op `json:"sar32"`
	ADD32stk  *U32Op `json:"add32_stack"`
	ADD32ixiy *U32Op `json:"add32_ixiy"`
	SUB32stk  *U32Op `json:"sub32_stack"`
	NEG32     *U32Op `json:"neg32"`
	CMP32zero *U32Op `json:"cmp32_zero"`
	ZEXT16_32 *U32Op `json:"zext16_32"`
	SEXT16_32 *U32Op `json:"sext16_32"`
	XOR32     *U32Op `json:"xor32_ixiy"`
	AND32     *U32Op `json:"and32_ixiy"`
	ROTR32    *U32Op `json:"rotr32"`
}

var (
	globalU32Ops     *U32OpsTable
	globalU32OpsOnce sync.Once
)

func GetU32OpsTable() *U32OpsTable {
	globalU32OpsOnce.Do(func() {
		globalU32Ops = loadU32OpsTable()
	})
	return globalU32Ops
}

func loadU32OpsTable() *U32OpsTable {
	paths := []string{
		os.ExpandEnv("$HOME/dev/z80-optimizer/data/u32_ops.json"),
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var table U32OpsTable
		if err := json.Unmarshal(data, &table); err != nil {
			fmt.Fprintf(os.Stderr, "[u32ops] parse error: %v\n", err)
			continue
		}
		count := 0
		if table.SHL32 != nil { count++ }
		if table.SHR32 != nil { count++ }
		if table.SAR32 != nil { count++ }
		if table.ADD32stk != nil { count++ }
		if table.NEG32 != nil { count++ }
		if table.CMP32zero != nil { count++ }
		if table.XOR32 != nil { count++ }
		if table.ROTR32 != nil { count++ }
		fmt.Fprintf(os.Stderr, "[u32ops] loaded %d u32 operations from %s\n", count, path)
		return &table
	}

	return &U32OpsTable{}
}

// ── Division table ──────────────────────────────────────────────────────────

type DivOpt struct {
	K        int      `json:"k"`
	Method   string   `json:"method"`   // "shift", "mul_shift", "mul_add256_shift"
	MagicM   *int     `json:"magic_m"`
	ShiftS   int      `json:"shift_s"`
	Ops      []string `json:"ops"`      // complete sequence
	Length   int      `json:"length"`
	TStates  int      `json:"tstates"`
	Bytes    int      `json:"bytes"`
	Clobber  []string `json:"clobbers"`
	Verified bool     `json:"verified"`
	Preamble       []string `json:"preamble"`
	PreambleTStates int     `json:"preamble_tstates"`
}

type DivOptTable struct {
	entries map[int]*DivOpt
}

var (
	globalDivOpt     *DivOptTable
	globalDivOptOnce sync.Once
)

func GetDivOptTable() *DivOptTable {
	globalDivOptOnce.Do(func() {
		globalDivOpt = loadDivOptTable()
	})
	return globalDivOpt
}

func loadDivOptTable() *DivOptTable {
	t := &DivOptTable{entries: make(map[int]*DivOpt)}

	paths := []string{
		os.ExpandEnv("$HOME/dev/z80-optimizer/data/div8_optimal.json"),
	}
	if p := os.Getenv("DIV8_PATH"); p != "" {
		paths = []string{p}
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		// Try wrapped format: {"entries": [...]}
		var wrapped struct {
			Entries []DivOpt `json:"entries"`
		}
		if err := json.Unmarshal(data, &wrapped); err == nil && len(wrapped.Entries) > 0 {
			for i := range wrapped.Entries {
				t.entries[wrapped.Entries[i].K] = &wrapped.Entries[i]
			}
			fmt.Fprintf(os.Stderr, "[divopt] loaded %d div8 entries from %s\n", len(wrapped.Entries), path)
			return t
		}
		// Fallback: flat array format
		var entries []DivOpt
		if err := json.Unmarshal(data, &entries); err != nil {
			fmt.Fprintf(os.Stderr, "[divopt] parse error: %v\n", err)
			continue
		}
		for i := range entries {
			t.entries[entries[i].K] = &entries[i]
		}
		fmt.Fprintf(os.Stderr, "[divopt] loaded %d div8 entries from %s\n", len(entries), path)
		return t
	}

	return t
}

func (t *DivOptTable) Lookup(k int) *DivOpt {
	if t == nil {
		return nil
	}
	return t.entries[k]
}

func (t *DivOptTable) Size() int {
	if t == nil {
		return 0
	}
	return len(t.entries)
}
