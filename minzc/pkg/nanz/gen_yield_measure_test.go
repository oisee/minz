package nanz_test

// G1 measurements (TASK "Measurements"): frame size, next() code bytes and
// Z80 T-states per element for the generated pull path, compared with the
// hand-written G0 cursors and with a plain hand-written while loop doing the
// same work inline (an approximate lower bound: it runs no pull protocol and
// keeps all state in registers/globals, which no iterator interface can).
// Every variant consumes each element into a returned u16 sum, so no read can
// be optimised away. The hand-written cursors keep their frames in globals
// (bc1/rc) while generated frames are stack allocas — the tables in PROGRESS
// note this where the numbers are quoted.
// Numbers are logged, not asserted: there is deliberately no performance
// gate here.

import (
	"sort"
	"testing"

	"github.com/minz/minzc/pkg/asmutil"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

const g1MeasureSrc = `
global data: [u8; 256]
global rle_in: [u8; 32]

struct ByteCursor {
    pos: ^u8,
    left: u8,
    done: u8,
}

struct RleCursor {
    src: ^u8,
    pairs: u8,
    left: u8,
    val: u8,
    done: u8,
}

global bc1: ByteCursor
global rc: RleCursor

gen bytes(src: ^u8, n: u8) -> u8 {
    while n != 0 {
        yield src[0]
        src = src + 1
        n = n - 1
    }
}

gen run_bytes(src: ^u8, pairs: u8) -> u8 {
    while pairs != 0 {
        var left: u8 = src[0]
        let value: u8 = src[1]
        src = src + 2
        pairs = pairs - 1
        while left != 0 {
            yield value
            left = left - 1
        }
    }
}

fun bc_make(cur: ^ByteCursor, src: ^u8, n: u8) -> void {
    cur.pos = src
    cur.left = n
    cur.done = 0
}

fun bc_next(cur: ^ByteCursor, out: ^u8) -> u8 {
    if cur.done != 0 { return 0 }
    if cur.left == 0 {
        cur.done = 1
        return 0
    }
    out^ = cur.pos^
    cur.pos = cur.pos + 1
    cur.left = cur.left - 1
    return 1
}

fun rle_make(cur: ^RleCursor, src: ^u8, pairs: u8) -> void {
    cur.src = src
    cur.pairs = pairs
    cur.left = 0
    cur.val = 0
    cur.done = 0
}

fun rle_next(cur: ^RleCursor, out: ^u8) -> u8 {
    if cur.done != 0 { return 0 }
    while cur.left == 0 {
        if cur.pairs == 0 {
            cur.done = 1
            return 0
        }
        cur.left = cur.src[0]
        cur.val = cur.src[1]
        cur.src = cur.src + 2
        cur.pairs = cur.pairs - 1
    }
    out^ = cur.val
    cur.left = cur.left - 1
    return 1
}

fun load_data(n: u8) -> void {
    for i in 0..n {
        data[i] = i * 37 + 11
    }
}

fun load_rle() -> void {
    var i: u8 = 0
    while i < 5 {
        rle_in[i * 2] = 51
        rle_in[i * 2 + 1] = 7
        i = i + 1
    }
}

// Baseline: argument setup, call and halt, without iteration.
fun bench_none(n: u8) -> u16 {
    return n
}

// The shared source-loading prologue, so its cost can be subtracted.
fun bench_load_bytes(n: u8) -> u16 {
    load_data(n)
    return 0
}

fun bench_load_rle() -> u16 {
    load_rle()
    return 0
}

// Generated pull frame, manual consumption. Each element is consumed into
// the returned sum so the read cannot be optimised away.
fun bench_gen_bytes(n: u8) -> u16 {
    load_data(n)
    var it = bytes(&data, n)
    var v: u8 = 0
    var sum: u16 = 0
    while pull_next(&it, &v) {
        sum = sum + v
    }
    return sum
}

// Hand-written cursor from the G0 oracle.
fun bench_man_bytes(n: u8) -> u16 {
    load_data(n)
    bc_make(&bc1, &data, n)
    var v: u8 = 0
    var sum: u16 = 0
    while bc_next(&bc1, &v) != 0 {
        sum = sum + v
    }
    return sum
}

// The same work inline: the approximate lower bound. The element read is
// consumed into the sum.
fun bench_inline_bytes(n: u8) -> u16 {
    load_data(n)
    var p: ^u8 = &data
    var left: u8 = n
    var sum: u16 = 0
    while left != 0 {
        var v: u8 = p^
        p = p + 1
        left = left - 1
        sum = sum + v
    }
    return sum
}

fun bench_gen_rle() -> u16 {
    load_rle()
    var it = run_bytes(&rle_in, 5)
    var v: u8 = 0
    var sum: u16 = 0
    while pull_next(&it, &v) {
        sum = sum + v
    }
    return sum
}

fun bench_man_rle() -> u16 {
    load_rle()
    rle_make(&rc, &rle_in, 5)
    var v: u8 = 0
    var sum: u16 = 0
    while rle_next(&rc, &v) != 0 {
        sum = sum + v
    }
    return sum
}

fun bench_inline_rle() -> u16 {
    load_rle()
    var p: ^u8 = &rle_in
    var pairs: u8 = 5
    var sum: u16 = 0
    while pairs != 0 {
        var left: u8 = p[0]
        let value: u8 = p[1]
        p = p + 2
        pairs = pairs - 1
        while left != 0 {
            left = left - 1
            sum = sum + value
        }
    }
    return sum
}
`

func TestG1GenYieldMeasurements(t *testing.T) {
	hm, err := nanz.Parse(g1MeasureSrc, "g1_measure")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	steps, err := pipeline.CompileHIRSteps(hm, pipeline.DefaultOptions())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	// Frame sizes: the expansion appends the precise frame structs to the
	// module it lowered.
	frameBytes := map[string]int{}
	for _, st := range hm.Structs {
		switch st.Name {
		case "bytes$frame", "run_bytes$frame", "ByteCursor", "RleCursor":
			frameBytes[st.Name] = mir2.ByteWidth(st)
		}
	}
	t.Logf("frame bytes: generated bytes$frame=%d (hand ByteCursor=%d), generated run_bytes$frame=%d (hand RleCursor=%d)",
		frameBytes["bytes$frame"], frameBytes["ByteCursor"], frameBytes["run_bytes$frame"], frameBytes["RleCursor"])

	// Code bytes of the continuation functions: the extent from a function's
	// entry label to the next function entry label in the assembled module.
	as := z80asm.NewAssembler()
	res, err := as.AssembleString(steps.Assembly)
	if err != nil || len(res.Errors) > 0 {
		t.Fatalf("assemble: %v %v", err, res.Errors)
	}
	funcLabels := map[string]bool{}
	for _, f := range steps.MIR2Module.Funcs {
		funcLabels[asmutil.SanitizeZ80Label(f.Name)] = true
	}
	type sym struct {
		name string
		addr int
	}
	var fnSyms []sym
	for n, a := range res.Symbols {
		if funcLabels[n] {
			fnSyms = append(fnSyms, sym{n, a})
		}
	}
	sort.Slice(fnSyms, func(i, j int) bool { return fnSyms[i].addr < fnSyms[j].addr })
	codeBytes := func(name string) int {
		for i, s := range fnSyms {
			if s.name == name {
				if i+1 < len(fnSyms) {
					return fnSyms[i+1].addr - s.addr
				}
				return res.Origin + len(res.Binary) - s.addr
			}
		}
		return -1
	}
	t.Logf("code bytes: bytes$next=%d (hand bc_next=%d), run_bytes$next=%d (hand rle_next=%d)",
		codeBytes("bytes_next"), codeBytes("bc_next"), codeBytes("run_bytes_next"), codeBytes("rle_next"))

	// T-states on the Z80 emulator for a fixed workload. The bootstrap
	// (LD SP, argument setup, CALL, HALT) is common to every measurement and
	// is reported separately via bench_none. bench_load_* also pays the
	// bootstrap, so only its net load cost is subtracted — the bootstrap
	// itself exactly once.
	const elems = 200
	measure := func(fn string, args ...int64) (int64, int64) {
		cycles, ret, err := pipeline.RunZ80Cycles(hm, steps.MIR2Module, steps.Allocation, steps.Assembly, fn, args)
		if err != nil {
			t.Fatalf("%s: %v", fn, err)
		}
		return cycles, ret
	}
	base, baseRet := measure("bench_none", elems)
	if baseRet != elems {
		t.Fatalf("bench_none returned %d", baseRet)
	}
	loadBytesCycles, _ := measure("bench_load_bytes", elems)
	loadRleCycles, _ := measure("bench_load_rle")
	netLoadBytes, netLoadRle := loadBytesCycles-base, loadRleCycles-base

	// Every variant returns the sum of the consumed elements; derive the
	// expected sums from the data pattern, u16 wrap-around included.
	var wantBytes uint16
	for i := 0; i < elems; i++ {
		wantBytes += uint16(byte(i*37 + 11))
	}
	wantRle := uint16(255 * 7)

	perElem := func(cycles int64, netLoad int64, n int64) int64 {
		return (cycles - base - netLoad) / n
	}
	for _, tc := range []struct {
		fn      string
		args    []int64
		want    uint16
		n       int64
		netLoad int64
	}{
		{"bench_gen_bytes", []int64{elems}, wantBytes, elems, netLoadBytes},
		{"bench_man_bytes", []int64{elems}, wantBytes, elems, netLoadBytes},
		{"bench_inline_bytes", []int64{elems}, wantBytes, elems, netLoadBytes},
		{"bench_gen_rle", nil, wantRle, 255, netLoadRle},
		{"bench_man_rle", nil, wantRle, 255, netLoadRle},
		{"bench_inline_rle", nil, wantRle, 255, netLoadRle},
	} {
		cycles, ret := measure(tc.fn, tc.args...)
		if uint16(ret) != tc.want {
			t.Errorf("%s returned %d, want %d", tc.fn, ret, tc.want)
		}
		t.Logf("%s: total T-states=%d (bootstrap %d + net load %d), per element=%d (approximate for the inline bound)",
			tc.fn, cycles, base, tc.netLoad, perElem(cycles, tc.netLoad, tc.n))
	}
}
