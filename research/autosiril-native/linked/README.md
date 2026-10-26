# One-image MIDI → paired notes → three flattened channels

This is the in-process alternative to ADR0053's sequential file utilities. The
unchanged pairing and flatten algorithms share one target arena; no host note
transfer connects the phases. All three actual Crystal scalar roots are bound
with isolated closure/effect/error-ABI checks. Only `native_main` is retained as
a named external entry, permitting ordinary private function pruning.

The first saved proof (`evidence/receipt.json`) used Chronos with chunk256. It
checks every one of the 1,485 paired notes and all 2,153 flattened events
(771/719/663) against original Ruby, in raw MIR2, final MIR2 and native Z80. Arena
bytes and native write counts must remain unchanged throughout flattening.

The image is 40,626 bytes: 20,313 code + 19,726 globals + 587 static spills. At
origin0x4000 this leaves only270 bytes before the existing stack-window guard;
it is not an established Spectrum disk-service memory layout. Capacities remain
2,046 notes / 512 active notes / 1,024 output entries per channel. No GC, physical
banking, file transport, full arrangement pipeline or VT2/VTX serializer is
provided by this composition.

To reproduce, first create fresh Ruby/Crystal reference material with the MIDI
gate. Outputs go outside the checkout:

```sh
python3 research/autosiril-native/midi/verify.py \
  --autosiril /path/to/autosiril --out /tmp/linked-reference
python3 research/autosiril-native/linked/compose.py --out /tmp/linked-probe
ruby research/autosiril-vm/oracle.rb /path/to/autosiril \
  /path/to/autosiril/test/chronos.mid > /tmp/linked-probe/ruby-full.json
python3 - <<'PY'
import json
from pathlib import Path
c = json.loads(Path('/tmp/linked-reference/cases.json').read_text())['cases'][0]
c['chunks'] = [1, 7, 256]
# The shared peak counter is reused by flattening. The linked phase hook checks
# parser peak2 and each channel's independent peak separately, before reuse.
c['expected'].pop('peak_active')
Path('/tmp/linked-probe/cases.json').write_text(json.dumps({'cases': [c]}))
PY
cd minzc
go run /tmp/linked-probe/linked-probe.go \
  -source /tmp/linked-probe/linked.nanz \
  -input /tmp/linked-probe/cases.json \
  -music /tmp/linked-reference/music.lanz \
  -manifest /tmp/linked-reference/music-manifest.json \
  -linked-oracle /tmp/linked-probe/ruby-full.json \
  -linked-records /tmp/linked-probe/phases.json \
  -out /tmp/linked-probe/measured
```

That command executes nine complete pipelines and records36 phase checks. Native
T-states exclude host OS services and wall-clock overhead. At roughly939 million
T-states for chunk256, the unoptimized algorithm is an offline proof, not a
real-time Spectrum converter or an assembly-quality benchmark.
