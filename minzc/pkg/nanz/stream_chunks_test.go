package nanz_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"testing"
)

func TestPacketDecoderEveryPartitionNative(t *testing.T) {
	source := `import mem.bytes
import stream.packet
import stream.bit_fields
@bit_fields("Header") { bits "kind" 3
    bits "flags" 5 }
global stream_bytes:[u8;8] = [163,2,0,11,22,93,0,0]
fun partition(mask:u8) -> u16 {
    var memory:[u8;4]
    var data:^u8 = &memory
    data[0] = 197
    data[3] = 197
    var body:ByteBuffer
    body.init(data + 1,2)
    var decoder:PacketDecoder
    decoder.init(&body)
    var report:FeedReport
    var input:ByteView
    var empty:ByteView
    var null:^u8 = 0
    empty.init(null,0)
    var output:ByteView
    var header:Header
    var packets:u8 = 0
    var start:u16 = 0
    var offset:u16 = 0
    while start < 8 {
        var end:u16 = start + 1
        while end < 8 {
            if (mask & (1 << (end - 1))) != 0 { break }
            end = end + 1
        }
        var old_phase:u8 = decoder.phase
        var old_length:u16 = decoder.length
        var old_count:u16 = body.len
        if decoder.feed(&empty,&report) != FeedStatus.NEED_MORE { return 1 }
        if report.consumed != 0 { return 2 }
        if decoder.phase != old_phase { return 3 }
        if decoder.length != old_length { return 4 }
        if body.len != old_count { return 5 }
        offset = start
        while offset < end {
            var base:^u8 = &stream_bytes
            input.init(base + offset,end - offset)
            let status:u8 = decoder.feed(&input,&report)
            let used:u16 = report.consumed
            if used == 0 { return 6 }
            if used > end - offset { return 7 }
            offset = offset + used
            if status == FeedStatus.READY {
                if packets >= 2 { return 8 }
                if decoder.view(&output) != 1 { return 9 }
                Header_decode(u16(decoder.prefix),&header)
                if packets == 0 {
                    if header.kind != 5 { return 10 }
                    if header.flags != 3 { return 11 }
                    if output.len != 2 { return 12 }
                    if output.data[0] != 11 { return 13 }
                    if output.data[1] != 22 { return 14 }
                    // Payload is now in decoder storage, independent of input.
                    base[3] = 99
                    if output.data[0] != 11 { return 15 }
                } else {
                    if header.kind != 2 { return 16 }
                    if header.flags != 29 { return 17 }
                    if output.len != 0 { return 18 }
                }
                let saved_prefix:u8 = decoder.prefix
                let saved_length:u16 = decoder.length
                let saved_count:u16 = body.len
                if decoder.feed(&input,&report) != FeedStatus.BLOCKED { return 19 }
                if report.consumed != 0 { return 20 }
                if decoder.prefix != saved_prefix { return 21 }
                if decoder.length != saved_length { return 22 }
                if body.len != saved_count { return 23 }
                if decoder.ack() != 1 { return 24 }
                packets = packets + 1
            } else {
                if status != FeedStatus.NEED_MORE { return 25 }
                if offset != end { return 26 }
            }
        }
        start = end
    }
    if packets != 2 { return 27 }
    if decoder.finish() != FeedStatus.DONE { return 28 }
    if decoder.feed(&empty,&report) != FeedStatus.DONE { return 29 }
    if report.consumed != 0 { return 30 }
    if data[0] != 197 { return 31 }
    if data[3] != 197 { return 32 }
    return 0
}
`
	m, err := nanz.ParseWithOpts(source, "stream-partitions.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	m.NonNullObjects = true
	for mask := int64(0); mask < 128; mask++ {
		m.Asserts = append(m.Asserts, hir.Assert{FuncName: "partition", Args: []int64{mask}, Expected: 0, Source: "every partition of two packets, empty chunks and blocked output"})
	}
	if err := hir.Verify(m); err != nil {
		t.Fatal(err)
	}
	if err := pipeline.RunAssertsMIR2(m, hir.LowerModule(hir.CloneModule(m))); err != nil {
		t.Fatal(err)
	}
	if _, err := pipeline.CompileHIRSteps(m, pipeline.Options{DisableVariantSearch: true}); err != nil {
		t.Fatal(err)
	}
}

func TestPacketDecoderEOFNative(t *testing.T) {
	source := `import mem.bytes
import stream.packet
global valid:[u8;5] = [163,2,0,11,22]
global oversize:[u8;4] = [163,255,255,99]
fun ending(length:u16) -> u16 {
    var memory:[u8;4]
    var data:^u8 = &memory
    data[0] = 197
    data[3] = 197
    var body:ByteBuffer
    body.init(data + 1,2)
    var decoder:PacketDecoder
    decoder.init(&body)
    var input:ByteView
    var report:FeedReport
    input.init(&valid,length)
    let status:u8 = decoder.feed(&input,&report)
    if report.consumed != length { return 1 }
    if length == 0 {
        if status != FeedStatus.NEED_MORE { return 2 }
        if decoder.finish() != FeedStatus.DONE { return 3 }
    } else {
        if length == 5 {
            if status != FeedStatus.READY { return 4 }
            if decoder.finish() != FeedStatus.READY { return 5 }
            if decoder.ack() != 1 { return 6 }
            if decoder.finish() != FeedStatus.DONE { return 7 }
        } else {
            if status != FeedStatus.NEED_MORE { return 8 }
            if decoder.finish() != FeedStatus.ERROR { return 9 }
            if decoder.error != PacketError.TRUNCATED { return 10 }
            if decoder.feed(&input,&report) != FeedStatus.ERROR { return 11 }
            if report.consumed != 0 { return 12 }
        }
    }
    decoder.reset()
    input.init(&oversize,4)
    if decoder.feed(&input,&report) != FeedStatus.ERROR { return 13 }
    if report.consumed != 3 { return 14 }
    if decoder.error != PacketError.TOO_LARGE { return 15 }
    if body.len != 0 { return 16 }
    if data[0] != 197 { return 17 }
    if data[3] != 197 { return 18 }
    decoder.reset()
    input.init(&valid,5)
    if decoder.feed(&input,&report) != FeedStatus.READY { return 19 }
    return 0
}
`
	m, err := nanz.ParseWithOpts(source, "stream-eof.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	m.NonNullObjects = true
	for n := int64(0); n <= 5; n++ {
		m.Asserts = append(m.Asserts, hir.Assert{FuncName: "ending", Args: []int64{n}, Expected: 0, Source: "EOF and sticky faults with recovery"})
	}
	if err := hir.Verify(m); err != nil {
		t.Fatal(err)
	}
	if err := pipeline.RunAssertsMIR2(m, hir.LowerModule(hir.CloneModule(m))); err != nil {
		t.Fatal(err)
	}
	if _, err := pipeline.CompileHIRSteps(m, pipeline.Options{DisableVariantSearch: true}); err != nil {
		t.Fatal(err)
	}
}

func TestPacketDecoderRingPumpNative(t *testing.T) {
	source := `import mem.bytes
import stream.packet
global pump_bytes:[u8;8] = [163,2,0,11,22,93,0,0]
fun probe(head:u16) -> u16 {
    var ring_memory:[u8;10]
    var payload_memory:[u8;4]
    var ring_data:^u8 = &ring_memory
    var payload_data:^u8 = &payload_memory
    ring_data[0] = 197
    ring_data[9] = 197
    payload_data[0] = 197
    payload_data[3] = 197
    var input:ByteRing
    var storage:ByteBuffer
    var decoder:PacketDecoder
    var report:FeedReport
    var out:ByteView
    var discarded:u8 = 0
    input.init(ring_data + 1,8)
    storage.init(payload_data + 1,2)
    decoder.init(&storage)
    var i:u16 = 0
    while i < head {
        if input.try_push(99) != 1 { return 1 }
        if input.try_pop(&discarded) != 1 { return 2 }
        i = i + 1
    }
    i = 0
    while i < 8 {
        if input.try_push(pump_bytes[i]) != 1 { return 3 }
        i = i + 1
    }
    if decoder.pump(&input,&report) != FeedStatus.READY { return 4 }
    if report.consumed != 5 { return 5 }
    if input.len != 3 { return 6 }
    if decoder.view(&out) != 1 { return 7 }
    if out.len != 2 { return 8 }
    if out.data[0] != 11 { return 9 }
    if out.data[1] != 22 { return 10 }
    let saved_head:u16 = input.head
    if decoder.pump(&input,&report) != FeedStatus.BLOCKED { return 11 }
    if report.consumed != 0 { return 12 }
    if input.head != saved_head { return 13 }
    if input.len != 3 { return 14 }
    if decoder.ack() != 1 { return 15 }
    if decoder.pump(&input,&report) != FeedStatus.READY { return 16 }
    if report.consumed != 3 { return 17 }
    if input.len != 0 { return 18 }
    if decoder.prefix != 93 { return 19 }
    if decoder.length != 0 { return 20 }
    if decoder.ack() != 1 { return 21 }
    if decoder.pump(&input,&report) != FeedStatus.NEED_MORE { return 22 }
    if report.consumed != 0 { return 23 }
    if decoder.finish() != FeedStatus.DONE { return 24 }
    if ring_data[0] != 197 { return 25 }
    if ring_data[9] != 197 { return 26 }
    if payload_data[0] != 197 { return 27 }
    if payload_data[3] != 197 { return 28 }
    return 0
}`
	m, e := nanz.ParseWithOpts(source, "packet-ring.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if e != nil {
		t.Fatal(e)
	}
	m.NonNullObjects = true
	for head := int64(0); head < 8; head++ {
		m.Asserts = append(m.Asserts, hir.Assert{FuncName: "probe", Args: []int64{head}, Expected: 0})
	}
	if e := hir.Verify(m); e != nil {
		t.Fatal(e)
	}
	if e := pipeline.RunAssertsMIR2(m, hir.LowerModule(hir.CloneModule(m))); e != nil {
		t.Fatal(e)
	}
	if _, e := pipeline.CompileHIRSteps(m, pipeline.Options{DisableVariantSearch: true}); e != nil {
		t.Fatal(e)
	}
}
