package nanz_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

// The oracle is a host FIFO slice; modulo is used only to track the physical
// backing image, independently of the target's subtraction-based wrap logic.
func TestByteRingImportedFIFO(t *testing.T) {
	for _, capacity := range []int{0, 1, 3, 255, 256} {
		for _, seed := range []int{0, 255} {
			t.Run(fmt.Sprintf("cap%d/seed%d", capacity, seed), func(t *testing.T) {
				var ops, values, statuses, reads, lengths, heads, firstLengths, pool []int
				image := make([]int, 258)
				for i := range image {
					image[i] = 197
				}
				var queue []int
				head := 0
				add := func(op, value int) {
					status, read := 0, 197
					switch op {
					case 0:
						if len(queue) > 0 {
							status = 1
							read = queue[0]
							queue = queue[1:]
							head = (head + 1) % capacity
						}
					case 1:
						if len(queue) < capacity {
							status = 1
							image[1+(head+len(queue))%capacity] = value
							queue = append(queue, value)
						}
					case 2:
						status = 1
						queue = nil
						head = 0
					case 3:
						status = 1
						pool = append(pool, queue...)
					}
					first := 0
					if len(queue) > 0 {
						first = capacity - head
						if first > len(queue) {
							first = len(queue)
						}
					}
					ops = append(ops, op)
					values = append(values, value)
					statuses = append(statuses, status)
					reads = append(reads, read)
					lengths = append(lengths, len(queue))
					heads = append(heads, head)
					firstLengths = append(firstLengths, first)
				}
				add(0, 0)
				for i := 0; i < capacity+1; i++ {
					add(1, (seed+i)&255)
				}
				add(3, 0)
				for i := 0; i < capacity/2+1; i++ {
					add(0, 0)
				}
				add(3, 0)
				for i := 0; i < capacity/2+2; i++ {
					add(1, (seed+91+i)&255)
				}
				add(3, 0)
				for i := 0; i < capacity+1; i++ {
					add(0, 0)
				}
				add(2, 0)
				add(1, 91)
				add(3, 0)
				add(2, 0)
				add(0, 0)
				observedQueueBytes := len(pool)
				if len(pool) == 0 {
					pool = []int{197}
				}
				var source strings.Builder
				source.WriteString("import mem.bytes\n")
				for _, table := range []struct {
					name, ty string
					vals     []int
				}{
					{"ops", "u8", ops}, {"values", "u8", values}, {"statuses", "u8", statuses}, {"reads", "u8", reads},
					{"lengths", "u16", lengths}, {"heads", "u16", heads}, {"first_lengths", "u16", firstLengths},
					{"queue_bytes", "u8", pool}, {"image", "u8", image},
				} {
					parts := make([]string, len(table.vals))
					for i, v := range table.vals {
						parts[i] = strconv.Itoa(v)
					}
					fmt.Fprintf(&source, "global %s: [%s; %d] = [%s]\n", table.name, table.ty, len(parts), strings.Join(parts, ","))
				}
				fmt.Fprintf(&source, `fun probe() -> u16 {
    var storage: [u8; 258]
    var data: ^u8 = &storage
    var ring: ByteRing
    var first: ByteView
    var second: ByteView
    var i: u16 = 0
    while i < 258 { data[i] = 197
        i = i + 1 }
    ring.init(data + 1, %d)
    var at: u16 = 0
    i = 0
    while i < %d {
        var byte: u8 = 197
        var status: u8 = 1
        if ops[i] == 0 { status = ring.try_pop(&byte) }
        if ops[i] == 1 { status = ring.try_push(values[i]) }
        if ops[i] == 2 { ring.clear() }
        if ops[i] == 3 {
            ring.segments(&first, &second)
            if first.len != first_lengths[i] { return 1 }
            if first.len + second.len != lengths[i] { return 2 }
            var j: u16 = 0
            while j < first.len {
                if first.data[j] != queue_bytes[at] { return 3 }
                j = j + 1
                at = at + 1
            }
            j = 0
            while j < second.len {
                if second.data[j] != queue_bytes[at] { return 4 }
                j = j + 1
                at = at + 1
            }
        }
        if status != statuses[i] { return 1000 + i }
        if byte != reads[i] { return 2000 + i }
        if ring.len != lengths[i] { return 3000 + i }
        if ring.head != heads[i] { return 4000 + i }
        if ring.cap != %d { return 5 }
        if ring.data != data + 1 { return 6 }
        i = i + 1
    }
    i = 0
    while i < 258 {
        if data[i] != image[i] { return 5000 + i }
        i = i + 1
    }
    return 0
}
`, capacity, len(ops), capacity)
				m, err := nanz.ParseWithOpts(source.String(), "ring-fifo.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
				if err != nil {
					t.Fatal(err)
				}
				m.NonNullObjects = true
				m.Asserts = []hir.Assert{{FuncName: "probe", Expected: 0, Source: "host FIFO and full backing image"}}
				if err := hir.Verify(m); err != nil {
					t.Fatal(err)
				}
				if err := pipeline.RunAssertsMIR2(m, hir.LowerModule(hir.CloneModule(m))); err != nil {
					t.Fatal(err)
				}
				if _, err := pipeline.CompileHIRSteps(m, pipeline.Options{DisableVariantSearch: true}); err != nil {
					t.Fatal(err)
				}
				t.Logf("%d operations, %d observed queue bytes", len(ops), observedQueueBytes)
			})
		}
	}
}
