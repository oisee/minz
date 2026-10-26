package nanz_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

func TestBitFieldMetaNative(t *testing.T) {
	source := `import stream.bit_fields
@bit_fields("Tiny") { bits "kind" 3
    bits "flags" 5 }
@bit_fields("Cross") { bits "a" 3
    bits "b" 6
    bits "c" 7 }
fun tiny(raw:u8, field:u8) -> u8 {
    var out:Tiny
    Tiny_decode(u16(raw), &out)
    if field == 0 { return out.kind }
    return out.flags
}
fun cross(raw:u16, field:u8) -> u8 {
    var out:Cross
    Cross_decode(raw, &out)
    if field == 0 { return out.a }
    if field == 1 { return out.b }
    return out.c
}
`
	m, err := nanz.ParseWithOpts(source, "bit-fields.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	for x := int64(0); x < 256; x++ {
		m.Asserts = append(m.Asserts, hir.Assert{FuncName: "tiny", Args: []int64{x, 0}, Expected: x >> 5}, hir.Assert{FuncName: "tiny", Args: []int64{x, 1}, Expected: x & 31})
	}
	for _, x := range []int64{0, 1, 127, 128, 255, 256, 257, 511, 512, 8191, 8192, 32767, 32768, 65534, 65535} {
		for field, want := range []int64{x >> 13, (x >> 7) & 63, x & 127} {
			m.Asserts = append(m.Asserts, hir.Assert{FuncName: "cross", Args: []int64{x, int64(field)}, Expected: want})
		}
	}
	if err := pipeline.RunAssertsMIR2(m, hir.LowerModule(hir.CloneModule(m))); err != nil {
		t.Fatal(err)
	}
	if _, err := pipeline.CompileHIRSteps(m, pipeline.Options{DisableVariantSearch: true}); err != nil {
		t.Fatal(err)
	}
	negatives := []struct{ body, diagnostic string }{
		{`bits "x" 0`, "field width must be 1..8"},
		{`bits "x" "-1"`, "field width must be 1..8"},
		{`bits "x" 9`, "field width must be 1..8"},
		{`bits "x" 256`, "field width must be 1..8"},
		{`bits "x" 3`, "bit fields must total 8 or 16"},
		{`bits "x" 4
bits "x" 4`, "duplicate bit field"},
		{`bytes "x" 8`, "expected bits name width"},
		{`bits "x;evil" 8`, "invalid bit schema identifier"},
	}
	for _, c := range negatives {
		_, err := nanz.ParseWithOpts("import stream.bit_fields\n@bit_fields(\"Bad\") {"+c.body+"}", "bad-bits.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err == nil || !strings.Contains(err.Error(), c.diagnostic) {
			t.Fatalf("%s: %v, want %s", c.body, err, c.diagnostic)
		}
	}
}

func TestPacketDecoderBasicNative(t *testing.T) {
	source := `import mem.bytes
import stream.packet
global input_bytes:[u8;6] = [163,2,0,85,204,255]
fun probe() -> u16 {
    var bytes:[u8;6]
    var data:^u8 = &bytes
    var body:ByteBuffer
    var input:ByteView
    var output:ByteView
    var decoder:PacketDecoder
    var report:FeedReport
    var i:u16 = 0
    while i < 6 { data[i] = 197
        i = i + 1 }
    body.init(data + 1,4)
    decoder.init(&body)
    input.init(&input_bytes,6)
    if decoder.feed(&input,&report) != FeedStatus.READY { return 1 }
    if report.consumed != 5 { return 2 }
    if report.status != FeedStatus.READY { return 3 }
    if decoder.prefix != 163 { return 4 }
    if decoder.length != 2 { return 5 }
    if decoder.view(&output) != 1 { return 6 }
    if output.len != 2 { return 7 }
    if output.data[0] != 85 { return 8 }
    if output.data[1] != 204 { return 9 }
    if decoder.feed(&input,&report) != FeedStatus.BLOCKED { return 10 }
    if report.consumed != 0 { return 11 }
    if decoder.finish() != FeedStatus.READY { return 12 }
    if decoder.ack() != 1 { return 13 }
    if decoder.finish() != FeedStatus.DONE { return 14 }
    if decoder.ack() != 0 { return 15 }
    if decoder.feed(&input,&report) != FeedStatus.DONE { return 18 }
    if report.consumed != 0 { return 19 }
    if data[0] != 197 { return 16 }
    if data[5] != 197 { return 17 }
    return 0
}
`
	m, err := nanz.ParseWithOpts(source, "packet-basic.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	m.NonNullObjects = true
	m.Asserts = []hir.Assert{{FuncName: "probe", Expected: 0, Source: "pending packet and ack"}}
	if err := hir.Verify(m); err != nil {
		t.Fatalf("%v\n%s", err, m.Dump())
	}
	if err := pipeline.RunAssertsMIR2(m, hir.LowerModule(hir.CloneModule(m))); err != nil {
		t.Fatal(err)
	}
	if _, err := pipeline.CompileHIRSteps(m, pipeline.Options{DisableVariantSearch: true}); err != nil {
		t.Fatal(err)
	}
}
