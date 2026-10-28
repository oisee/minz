package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
)

// TestBitStorePointerHomeE2E: the fused SET/RES store is only encodable
// through (HL) at offset 0/1 and (IX+d)/(IY+d) with d in 0..127. Four live
// words push the pointer into DE, where the fusion used to be dropped after
// the load and the Or|And producer had already been suppressed, so the plain
// store wrote whatever the accumulator held.
func TestBitStorePointerHomeE2E(t *testing.T) {
	const source = `global buf: [u8; 4]
fun f(i: u8) -> u16 {
    var a: u16 = u16(i)
    var b: u16 = a + 1
    var c: u16 = a + 2
    var d: u16 = a + 3
    var p: ^u8 = buf
    p = p + u16(i)
    buf[0] = 0x0F
    p^ = p^ | 0x04
    a = a + b
    b = b + c
    return a + b + c + d + u16(p^)
}
assert f(0) == 24
`
	if stats := runNanzAsserts(t, source, "bit_store_de_home.nanz"); stats != (hir.AssertStats{Executed: 2, Passed: 2}) {
		t.Fatalf("assert stats = %+v", stats)
	}
}

// TestBitStorePointerHomeClearE2E: the And ~(1<<n) half of the same fusion.
func TestBitStorePointerHomeClearE2E(t *testing.T) {
	const source = `global buf: [u8; 4]
fun g(i: u8) -> u16 {
    var a: u16 = u16(i)
    var b: u16 = a + 1
    var c: u16 = a + 2
    var d: u16 = a + 3
    var p: ^u8 = buf
    p = p + u16(i)
    buf[0] = 0xFF
    p^ = p^ & 0xF7
    a = a + b
    b = b + c
    return a + b + c + d + u16(p^)
}
assert g(0) == 256
`
	if stats := runNanzAsserts(t, source, "bit_store_de_clear.nanz"); stats != (hir.AssertStats{Executed: 2, Passed: 2}) {
		t.Fatalf("assert stats = %+v", stats)
	}
}

// TestBitStorePointerHomeOffsetE2E: an offset pointer that lands outside the
// indexable window must keep the ordinary load-modify-store sequence.
func TestBitStorePointerHomeOffsetE2E(t *testing.T) {
	const source = `global buf: [u8; 4]
fun h(i: u8, o: u16) -> u16 {
    var a: u16 = u16(i)
    var b: u16 = a + 1
    var c: u16 = a + 2
    var d: u16 = a + 3
    var p: ^u8 = buf
    p = p + u16(i) + o
    buf[0] = 0x08
    buf[1] = 0x08
    p^ = p^ | 0x04
    a = a + b
    b = b + c
    return a + b + c + d + u16(p^)
}
assert h(0, 0) == 21
assert h(0, 1) == 21
`
	if stats := runNanzAsserts(t, source, "bit_store_offset.nanz"); stats != (hir.AssertStats{Executed: 4, Passed: 4}) {
		t.Fatalf("assert stats = %+v", stats)
	}
}

// TestBitStoreWordPointerHomeE2E: a u16 cell whose modified bit lives in the
// high byte (byteOffset 1) is only encodable from HL and IX/IY.
func TestBitStoreWordPointerHomeE2E(t *testing.T) {
	const source = `global wbuf: [u16; 4]
fun w(i: u8) -> u16 {
    var a: u16 = u16(i)
    var b: u16 = a + 1
    var c: u16 = a + 2
    var d: u16 = a + 3
    var p: ^u16 = wbuf
    p = p + u16(i)
    wbuf[0] = 0x0101
    p^ = p^ | 0x0200
    a = a + b
    b = b + c
    return a + b + c + d + p^
}
assert w(0) == 778
`
	if stats := runNanzAsserts(t, source, "bit_store_word_home.nanz"); stats != (hir.AssertStats{Executed: 2, Passed: 2}) {
		t.Fatalf("assert stats = %+v", stats)
	}
}
