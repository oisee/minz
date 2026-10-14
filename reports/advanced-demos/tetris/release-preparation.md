# Pixel-step Tetris: main and six-host release preparation

14 October 2026. Candidate branch: `feat/advanced-demos`; gameplay/rendering
checkpoint: `456eb722`. This is preparation for demo packages **0.25.1**,
not a published release or an integration claim.

The playable main uses declaratively baked masked sprite instances, pixel gravity,
phase-aware movement/rotation/locking and shadow publication. The current
[preparation report](native-preparation.md) records full native/VM proofs and
limits: 969 fetched main field frames across three modes without mixed images,
1596 independent physics cases per mode and measured dirty-row restoration.
Stable 50 FPS is not established. TV effects and remaining advanced demos remain
part of the broader objective after this nearest milestone.

## Integration checks

At checkpoint 456eb722, local `go test -short -timeout 30m` passes for z80effects,
HIR, MIR2, Z80 assembler, C89, codegen, parser, PL/M, Lanz and Lizp. MIR2 takes
111.945 seconds in this run. Nanz language regressions with only the separate
showcase compile/assemble inventory excluded pass in 12.593 seconds.
The existing final fast pipeline receipt is 394/394; deep release checks and
required PR checks must still pass on the submitted candidate.

## Packages

The manual Demo platform packages workflow now builds **all six** executables
from its selected Git ref and version: Linux amd64/arm64, Windows amd64/arm64,
macOS Intel/Apple Silicon. Linux builds use Debian Bookworm and native hosts;
GUI startup verification uses Xvfb. macOS bundles receive ad-hoc signatures.
Every host runs package SHA/ZIP checks, all six demo startups and Zork session/
SAVE/RESTORE checks. Those startup receipts complement the independent Tetris
machine proofs; they do not establish physical Spectrum timing.

Packages include the Tetris source, launcher and builder plus the `hw`/`zx`
libraries, including their compile-time sprite/scene generators. Version is an
explicit workflow input; packaging does not publish or replace old releases.

A local packaging-only smoke with the already-preserved checkpoint executable
verifies 53 manifest entries, exact ZIP contents, the final embedded Tetris
snapshot and library source bytes. It is labeled `0.25.1-packaging-smoke` and is
not a fresh release executable. Native-host builds, main merge and publication
remain pending.

Local logs and the packaging smoke are preserved under
`/home/alice/dev/minz-artifacts/grace-reports/2026-10-14/demo-release-preparation/`;
the corresponding share directory is `/mnt/safe/minz/grace/2026-10-14/demo-release-preparation/`.

## Six native hosts — verified candidate

[Run 38639893958](https://github.com/oisee/minz/actions/runs/38639893958)
passes on all six native runners. Packages are built at `826c341d` and remain
release candidates pending integration and the deep audit. Every downloaded
package was independently checked against its manifest, ZIP contents and the
current Tetris snapshot. All nine control PNG files agree byte-for-byte across
Linux amd64/arm64, Windows amd64/arm64 and macOS Intel/Apple Silicon.

Tetris field/actor/preview are visible. SAVE and fresh RESTORE both report `Ok`;
the following `look` shows the expected Zork II room. Native platform startup
does not replace the independent gameplay and ULA proofs, nor does it establish
physical Spectrum/Pi/Parallels operation.

Candidate ZIPs and `SHA256SUMS` are in
`/mnt/safe/minz/releases/v0.25.1-candidate/`. Complete native build metadata,
GUI receipts, screenshots and source/package hashes are preserved under the
local/share `demo-release-preparation/native/` directories above. The other
five demo snapshots are unchanged from 0.25.0; this candidate updates Tetris.

Advisory lint found unchecked setup/export errors and style suggestions in
test files. They are being corrected with a focused repeat of all 26 affected
tests. This follow-up changes the test harness only; it does not change the
compiler, emulator, library sources or embedded demo assets used by the six
native packages. Required CI and the full deep audit remain pending.


## Audit follow-up and direct-rendering scope

The 26 lint-affected harness tests pass (MIR2 0.095 s, pipeline 338.678 s).
PR CI at `9a2bd171` passes all required checks and advisory lint; its advisory
output sweep reports 10 output differences. The only exit change is an
improvement: old main panics on Tetris, while the candidate compiles it.
The archived comparison spans 838 source files; output identity is not a
semantic test of deliberately changed code generation. The earlier deep run
38639897937 passes nine pipeline shards and fails three. Exact keyboard
oracles omitted a SYMBOL SHIFT read already present on main; the SMC oracle
omitted newly callable typed setters. All four affected families pass locally
in 5.635 s. These test corrections are committed in `064dbc7b`.

The separate Sol audit reproduces all 66 long-fuzz discrepancies identically
on main and the feature branch. The Python oracle inferred the wrong width
for literal arithmetic. Corrected literal/widest-operand semantics pass all
10,000 seeds and all 20,000 MIR2/Z80 execution receipts; the independently
checked correction is integrated as `2ab7fd31`. Full audit data is preserved
under `demo-release-preparation/sol-fuzz/` on the share. The original failing
receipts remain available; this does not turn the old CI run green.

Alice then requested direct screen drawing, paired baked exact clearing,
two gravity rates (one pixel per one/two frames), and a real 50-Hz recording.
The six native packages above contain the earlier shadow renderer and remain
historical candidates. They do not satisfy this newer product scope. See
[the paired-sprite work](../paired-sprites/README.md); the Tetris migration,
menu, frame timing and video capture still require verification.
