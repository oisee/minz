package nanz_test

// E2E test for struct-based arena allocator.
// Compiles Nanz → MIR2, runs VM asserts (sizeof, split, alloc, sandbox).

import (
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

const arenaSrc = `
struct Arena {
    ptr: u16
    end: u16
}

struct Enemy {
    x: u8
    y: u8
    hp: u8
    sprite: u8
}

fun Arena.init(self: ^Arena, base: u16, size: u16) {
    self.ptr = base
    self.end = base + size
}

fun Arena.alloc(self: ^Arena, n: u16) -> u16 {
    let result: u16 = self.ptr
    let next: u16 = result + n
    if next > self.end {
        return 0
    }
    self.ptr = next
    return result
}

fun Arena.reset(self: ^Arena, base: u16) {
    self.ptr = base
}

fun Arena.remaining(self: ^Arena) -> u16 {
    return self.end - self.ptr
}

fun arena_split(a: ^Arena, start: u16, size: u16) -> u16 {
    a.init(start, size)
    return start + size
}

// --- Test: sizeof ---

fun test_sizeof() -> u16 {
    if sizeof(Arena) != 4 { return 1 }
    if sizeof(Enemy) != 4 { return 2 }
    if sizeof(u8) != 1 { return 3 }
    if sizeof(u16) != 2 { return 4 }
    return 0
}

// --- Test: split + typed alloc ---

global perm: Arena
global level: Arena
global frame: Arena

fun test_split_and_alloc() -> u16 {
    let next = arena_split(&perm,  0xC000, 256)
    let next2 = arena_split(&level, next,  2048)
    let next3 = arena_split(&frame, next2, 1024)

    if next != 0xC100 { return 1 }
    if next2 != 0xC900 { return 2 }
    if next3 != 0xCD00 { return 3 }

    let e = perm.alloc(sizeof(Enemy))
    if e != 0xC000 { return 4 }

    let e2 = perm.alloc(sizeof(Enemy))
    if e2 != 0xC004 { return 5 }

    let tmp = frame.alloc(64)
    if tmp != 0xC900 { return 6 }

    frame.reset(0xC900)
    let tmp2 = frame.alloc(32)
    if tmp2 != 0xC900 { return 7 }

    return 0
}

// --- Test: OOM ---

global a: Arena

fun test_oom() -> u16 {
    a.init(0xC000, 8)
    let p1 = a.alloc(4)
    if p1 != 0xC000 { return 1 }
    let p2 = a.alloc(4)
    if p2 != 0xC004 { return 2 }
    let p3 = a.alloc(1)
    if p3 != 0 { return 3 }
    return 0
}

assert test_sizeof() == 0 via mir2
assert test_split_and_alloc() == 0 via mir2
assert test_oom() == 0 via mir2

// Sandbox: sequential allocs share state
fun init_a() -> u16 {
    a.init(0xC000, 1024)
    return a.remaining()
}

fun alloc_enemy() -> u16 {
    return a.alloc(sizeof(Enemy))
}

sandbox "sequential" {
    assert init_a() == 1024 via mir2
    assert alloc_enemy() == 0xC000 via mir2
    assert alloc_enemy() == 0xC004 via mir2
    assert alloc_enemy() == 0xC008 via mir2
}
`

func TestArena_E2E(t *testing.T) {
	hm, err := nanz.Parse(arenaSrc, "arena_e2e")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// CompileHIRSteps runs MIR2 VM asserts + sandbox asserts internally.
	// If it returns nil, all asserts passed.
	_, err = pipeline.CompileHIRSteps(hm)
	if err != nil {
		t.Fatalf("compile (asserts failed): %v", err)
	}
	t.Logf("Arena E2E: %d top-level asserts + %d sandboxes passed",
		len(hm.Asserts), len(hm.Sandboxes))
}
