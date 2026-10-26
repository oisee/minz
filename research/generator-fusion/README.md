# Immediate generator fusion: experimental G2 profile

After PR #255 established bounded lazy Autosiril flattening, this checkpoint
removes pull overhead for one narrowly admitted immediate consumer. It is an
opt-in common HIR transformation, exposed as `--fuse-generators` in `mz` and
`FuseGenerators` in pipeline/lowering options. Default compilation is unchanged.

```nanz
gen stream(src:^u8,n:u8)->u8 {
    while n != 0 {
        yield src[0]
        src = src + 1
        n = n - 1
    }
}
fun main()->u16 {
    var total:u16 = 0
    for value in stream(&data,16) { total = total + u16(value) }
    return total
}
```

## Measured result

The same 16 runtime-loaded bytes (`byte(i*37+11)`) sum to 1800 in assembled
binaries executed by the Z80 CPU emulator. The shared 16-byte data array is
excluded from code size; all helper code is included. This is not physical-board
timing or a universal language benchmark.

| Variant | Code | Linked binary | T-states | Stack below entry |
|---|---:|---:|---:|---:|
| Direct Nanz loop | 32 B | 48 B | 1,240 | 2 B |
| Existing generator pull | 215 B | 231 B | 13,459 | 17 B |
| Opt-in immediate fusion | 30 B | 46 B | 1,586 | 0 B |

The fused kernel eliminates the producer frame, output slot and `$next` calls.
It is about 7.2 times smaller and 8.5 times faster than this pull kernel. It is
still slower than this direct loop; allocation/instruction selection has room
to improve. Stack numbers exclude the initial two-byte return sentinel. Programs
whose consumer explicitly takes an element's address can still need local storage.

## Admission and semantics

The first profile admits a producer with exactly one structured `while`: a
leading scalar yield followed by at most four assignments to its own parameters.
Parameters are scalar/pointer values; elements are u8/i8/u16/i16/bool. The generator
has one syntactic immediate consumption site and no manual cursor/reference uses.
Multiple consumers, prefix statements, producer locals, multiple yield sites and
other producer control stay on the existing pull path.

Arguments are captured once, left-to-right, under fresh private names. The consumer
is spliced at yield. Break abandons the producer before its suffix; return leaves
the enclosing function; consumer continue executes the producer suffix before
rechecking its condition. Break/continue belonging to nested consumer loops retain
their targets. Calls, volatile accesses and aliasing reads keep their order.
Consumer switch/goto/labels/assembly/nested generators and unknown nodes refuse
admission. This is not generalized ADR 0050 fusion.

HIR does not yet give every reference a resolved binding ID. Free producer global
and direct-call names must not collide with any caller binding (including loop
variables and captures); such sites conservatively retain pull. The same check
prevents calling a caller-local function alias in place of a producer's global
callee. Producer parameter addresses are not admitted. Reflective deep cloning
retains type, volatility and call metadata; it does not use textual substitution.

Explicit HIR loop policies, profiling/site collection and explicit variant policies
cannot currently combine with this flag; public pipeline APIs return a controlled
error. Automatic variant search is disabled while the flag is selected, including
sites that retain pull. No general cost selection, lifetime/provenance analysis or
hardware bank mapping is added. ADR 0048's existing raw-pointer lifetime limitation
remains. The complex/manual-pull Autosiril flattened producer is not fused yet.

## Reproduce

```sh
python3 research/generator-fusion/verify.py --out /tmp/minz-generator-fusion-proof

# Direct CLI use:
go -C minzc build -o /tmp/mz-generator-fusion ./cmd/minzc
/tmp/mz-generator-fusion research/generator-fusion/sum.nanz \
  -b z80 -t cpm --asserts none --fuse-generators -o /tmp/sum-fused.a80

# Focused semantic/native gate:
MINZ_GEN_FUSION_OUT=/tmp/minz-generator-fusion-proof \
  go -C minzc test ./pkg/pipeline -run '^TestGeneratorFusion' -count=1 -v
```

The verifier also checks the CLI's ordinary pull product and the manual-cursor
fallback, and records source/compiler hashes plus exact measurement options.
Generated outputs stay outside the checkout. `evidence/` retains receipts and
representative native assembly for review.

Tests compare pull/fused execution on raw MIR2, optimized MIR2 and assembled Z80:
full/empty consumption, early break/return, outer and nested continue/break,
backing-memory mutation, captured argument mutation, parameter/name hygiene,
left-to-right side-effectful arguments, condition/yield/consumer/suffix traces,
an unselected condition trap, signed/unsigned elements and bool. Separate VM
controls retain bounds/division diagnostics even when a yielded value is unused;
raw-pointer availability remains a VM sandbox policy, not a native hardware trap. Other controls
cover manual/multiple uses, producer refusal, free-global/call-alias collisions,
malformed captures and policy rejection without panic. Native cases check return
PC/SP, observable counters and stack canaries. All new focused tests are registered
in the stable pipeline inventory.
