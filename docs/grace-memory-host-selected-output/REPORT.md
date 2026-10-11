# Memory-host CLI saves the selected output format

The legacy `--memory-host-arena-append` command previously wrote assembly text to
`-o` regardless of its extension. `-o parser.bin` was therefore not a binary.
The same bug existed in the fixed-policy and joint policy/mask search commands.
All three now use one small output helper: `.bin` writes exactly the selected
candidate's BIN, `.a80` writes exactly its ASM. Other extensions are rejected by
shared admission before source-profile collection or measurement. Uppercase
`.BIN` and `.A80` are accepted.

No candidate is recompiled to create the output. The existing evidence-directory
writing behavior is unchanged; this patch does not extend atomic directory
publication to these memory-host commands.

## Actual collect/replay

From the repository root, with a single frozen `mz` executable:

```sh
mz examples/nanz/self_lanz_parser.nanz --target cpm \
  --memory-host-arena-append ./parser-collect \
  --memory-host-workload docs/grace-parser-prospective-heldout/artifacts/training-workload.json \
  -o ./parser.bin
mz examples/nanz/self_lanz_parser.nanz --target cpm \
  --memory-host-arena-append ./parser-replay \
  --memory-host-workload docs/grace-parser-prospective-heldout/artifacts/training-workload.json \
  --memory-host-profile ./parser-collect/source-profile.json \
  -o ./parser.a80
```

Both real CLI invocations select append-copy mask 2: **992 049 T-states, 2 344
bytes**, with identical selected binaries. The full three previously frozen
training inputs execute the unchanged self Lanz parser. This is a CLI correctness
repeat, not a fresh heldout experiment.

`audit.py` independently models parser syntax, complete global/resource state and
ordered byte events for every candidate in collect and replay. It also checks the
external BIN/ASM files byte for byte against the winning candidate and verifies
that replay selects the same binary. All checks pass.

The historical incorrect output remains unchanged in
`../grace-compiler-mode-artifacts/artifacts/parser-output-before-fix.bin`.
The audit confirms that it is ASM and differs from the corrected BIN; the old
negative evidence is not rewritten.

The native collect/replay regression uses the complete parser source with two
bounded zero-length calls and an independent oracle: `region_match(...,0)=1`,
`intern_region(...,0)=0x9000`. It checks actual `.bin` collection and `.a80` replay
against selected evidence. Focused native/admission checks pass in **42.872 s**;
policy/search admission checks pass in **0.103 s**. Additional extension tests
require the specific output-extension error before measurement.

All candidates' source/profile/ASM/BIN/MIR/allocation evidence and CLI logs are
saved here. Frozen executable:
`/home/alice/dev/minz-artifacts/grace-checkpoints/mz-memory-host-selected-output`,
SHA256 `2c9f5ee900ef909ba662e4242761f08199b8a99c8b19f8d3bdaf54834336ee75`.
The independently audited output is a measured assembled image, not a linked
CP/M COM application.
