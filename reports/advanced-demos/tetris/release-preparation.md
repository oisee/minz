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
