# First bounded native Autosiril flatten probe

Work in progress on `spike/autosiril-native-flat`, based on the forwarding-cycle
cleanup in PR #258. This is a diagnostic checkpoint, not the completed native
stage verification matrix or a complete Autosiril converter.

The probe compiles the actual `research/autosiril-vm/lazy_flatten.nanz` algorithm
with the actual exported Crystal `cell_action` and `prefer` functions. It selects
one 4-KiB arena bank (682 six-byte records), keeps the original 512 active handles,
and replaces only the host verification sink with Nanz output arrays. Musical
selection, iteration, note reads and output storage all execute in target code.
The original stage file is unchanged; bounded source substitutions are currently
an explicit research harness and need a checked configuration interface.

Four supplied intervals include overlap and a zero-length note. All eight emitted
row/value pairs and the fault/count/peak-active/row/yield/read/ownership counters
match raw MIR2, final MIR2 and assembled Z80. The native call returns to the
sentinel with restored SP and no stage fault.

Observed checkpoint:

- Linked native image: 13,083 bytes, including 6,437 bytes of declared MIR globals.
- Image remainder (code/helpers and any backend storage): 6,646 bytes.
- Observed stack below entry: 57 bytes, excluding the two-byte return sentinel.
- Z80 emulator time: 891,410 T-states.
- Origin 0x4000, image end 0x731b, entry SP 0xeffe, returned SP 0xf000.

These are one input and one bounded flat arena configuration. MIDI parsing,
full-song coverage, VT2 writing and physical bank switching are outside this
probe. Fresh Ruby comparison, rebuilt export provenance, capacity/early-stop
controls, read-only/input/stack canaries and a full receipt remain required for
the native-stage milestone. The existing exported products used here are real
Crystal code but are not claimed to have been freshly rebuilt by this probe.

## Run

From the repository root, supply actual native exporter products (not fixture
selector stubs):

```sh
go -C minzc run ../research/autosiril-native/probe.go \
  /path/to/music.lanz /path/to/music-manifest.json
```

The helper currently writes its assembled text to
`/tmp/minz-native-flat-probe.a80`. `evidence/first-probe.txt` retains the observed
output and `first-probe.a80` the generated assembly. Next work is a proper native
runner/verifier using fresh Crystal and Ruby products with multiple inputs.
