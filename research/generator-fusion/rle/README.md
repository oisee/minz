# Checked nested generator fusion: streaming RLE

The opt-in common HIR pass now admits two nested producer whiles with a single
leading inner yield. This covers `[count,value]` RLE runs, including zero-length
runs. Prefix scalar locals are initialized when the outer body actually runs;
parameters and locals get fresh names. Assignments only update private producer
state. Suffixes contain at most four assignments per loop; loop bodies contain at
most twelve statements. Deeper loops, duplicate/shadowed producer locals, local
addresses, volatile/absolute producer locals, producer stores and other control
retain pull lowering.

```nanz
gen runs(src:^u8,pairs:u8)->u8 {
    while pairs != 0 {
        var left:u8 = src[0]
        let value:u8 = src[1]
        src = src + 2
        pairs = pairs - 1
        while left != 0 {
            yield value
            left = left - 1
        }
    }
}
```

The consumer executes at the yield, inside the inner loop. Consumer `continue`
runs the suspended inner suffix and resumes its condition. Consumer `break`
sets a private stop flag, leaves the inner loop and immediately leaves the outer
loop before its suffix or next condition. The flag is omitted when no such break
exists. Nested consumer loops retain their own exits; `return` leaves the caller.
No intermediate decompressed buffer, generator frame or per-byte resume call is
needed at admitted sites. The input remains a caller-owned pointer, with the
existing raw-pointer lifetime contract.

## Executed native result

Eight runtime-loaded pairs decode to thirteen bytes with sum 1362. Complete
linked Z80 code includes helpers and excludes only the sixteen input bytes.

| Variant | Code | Linked binary | T-states | Stack below entry |
|---|---:|---:|---:|---:|
| Existing pull | 311 B | 327 B | 16,201 | 19 B |
| Checked fusion | 73 B | 89 B | 3,576 | 4 B |

This kernel is 4.3 times smaller and 4.5 times faster. Timing is from the Z80
emulator, not a physical board. Stack excludes the initial return sentinel.
The cost kernel fully consumes the stream and therefore needs no stop flag.
This is a measured case, not a general profitability guarantee.

Fourteen semantic fixtures compare against independent Go demand semantics:
full/empty/all-zero runs; 255-byte runs and byte-counter wrap; first-item break;
break after an empty run; an unrequested next-pair division trap; filtering before
taking two accepted outputs; continue;
return; nested consumer break/continue; backing-memory mutation; and caller-local
hygiene. Every fixture runs pull/fused on raw MIR2, final MIR2 and assembled Z80
(**84 executions**), comparing results, counters and the complete ordered event
trace. PC, restored SP and stack canaries are checked. Separate controls refuse
unsafe producer shapes, and the original one-loop regression suite still passes.
Two additional fixtures use pure loop conditions, allowing stronger loop
optimizations: break on the last byte of a run, and continue across the last byte
and a zero-length run. Both run on all six paths (12 more executions). Native CPU
read heatmaps assert that the break reads only the first pair; subsequent pairs
are never fetched. The fully consumed cost kernel reads every input byte once.

Filtering is expressed directly in the consumer for now:

```nanz
var accepted:u8 = 0
for v in runs(&data,8) {
    if v == 0 { continue }
    total = total + u16(v)
    accepted = accepted + 1
    if accepted == 2 { break }
}
```

This tests the demand contract; it does not add a fluent generator `filter/take`
API. It does not fuse the existing complex/manual-pull Autosiril producer, add
bank switching, or change the default compilation mode.

## Reproduce

```sh
python3 research/generator-fusion/rle/verify.py --out /tmp/minz-generator-rle-proof
```

The script rebuilds the compiler, runs the native semantic/cost gate, checks both
CLI products, and records source/compiler hashes, exact input and options.
`evidence/` retains native assembly, costs, test output and the receipt.
