# Production checked local table bridge

Base `6be9d4d4`. This connects validated local register tables to ordinary
Nanz/HIR → MIR → production Z80 compilation with explicit size-only admission.

## Audit findings

The base pipeline has no VIR import or CodegenModule call: both public ordinary
HIR compilation paths allocate through PBQP and call MIR2 Z80Codegen. The
comment describing VIR as an explicit production opt-in was stale.

VIR's research module PFCCO return conventions are informational. Its ordinary
appendReturnMove hardcodes A/HL, and block-param return inference can skip the
required move. LowerFunc also does not include OpRet; MIR terminators are emitted
outside the former local-key domain. These are reasons to use a narrow bridge
rather than replace the whole production module with research VIR.

Independent native audit reproduces full-F loss: CP 0 with A=1 returns F=02,
while legacy cleanup substitutes OR A and returns F=00. Z is preserved, N is not.
Flag certificates now require raw emission. Production leaf candidates bypass
all text peepholes, and reject comparison/flag operations until covered.

## Route and admitted scope

CLI:

```
minzc source.nanz --Osize --checked-local-table table.json -o program.a80
```

Options.CheckedLocalTablePath is explicit. Table input is loaded freshly once
per compilation, independently of research global tables. No path means the
unchanged production baseline. Missing/malformed explicit input is an error;
absent/invalid candidates retain baseline assembly. The table route requires
emitted Z80, OptSize and no profile selection; speed optimization remains
available through its existing route.

PrepareLocalTableProblem accepts one leaf block with a single unconditional
return, 8/16-bit arithmetic/moves/constants, and no effects/calls/CFG/flag return.
It copies instructions rather than mutating production MIR. An explicit return
move uses mir2.ReturnLocation from the actual contract (C, A, DE, HL, etc.),
followed by a checked OpRet. PBQP parameter locations are pinned. Returned block
parameters are deliberately rejected until independently covered.

The key includes the full MIR function dump and serialized Func.Contract
(including clobbers and FlagCond), terminator, actual return location, parameter
ABI and raw-emission domain in addition to the complete local schema metadata.
Wrong ABI or changed context cannot reuse the entry. Production caller contracts
and conservative assembly clobber headers remain intact.

After normal production contracts/allocation/codegen, each admitted candidate
must assemble strictly smaller than the baseline function and also reduce the
complete assembled module. At equality or any failure the baseline remains.
Final ordinary Z80 assertion judges run against the selected module as usual.
No solver, adapter, helper or extra storage is introduced by the candidate.
Selection uses actual BIN length, not table timing metadata. Pattern cost is
still recorded as a model, and is not claimed as exact post-generation timing.

## Concrete ordinary-source result

Source: `source.nanz`, add5(x: u8) returns x+5; main exercises add5(7).
A locally generated entry is saved as table.json, consumed through actual
CompileHIRSteps and the CLI, with exhaustive independent byte-result checking.

| Artifact | Module bytes | add5 fixture T-states |
|---|---:|---:|
| Baseline production | 8 | 56 |
| Checked local candidate | 6 | 49 |

All 256 byte inputs return `(x+5) mod 256`; SP restores to f000. Timing includes
the same LD A,x / CALL / DI / HALT fixture overhead. The candidate removes a
redundant materialized constant LD C,5; ADD A,5 and RET remain. CLI and selected
BIN match exactly: `3e0cc9c605c9`. This is one ordinary-source function benefit,
not a workload-wide or corpus performance claim.

Tampered cost, changed same-path table contents and changed return/clobber ABI
produce rejection or baseline fallback. Unsupported calls/CFG/flag return and
block-param return have negative controls. C-return native execution is checked
against C=37 and restored SP, independent of A/HL conventions.

## Checks and limitations

- Full VIR: PASS (`vir-full.txt`).
- New pipeline native/fallback tests and retired profile config check: PASS.
- Supported module compile-only checks: PASS (`compile.txt`).
- Actual CLI compile and exact ASM/BIN packet: PASS.
- Existing TestMultiFunctionProfileVMZ80 is red at this base: long_sum observes
  1690 T versus its fixed 1455 expectation. Pristine base pipeline/profile files
  reproduce the same failure (`profile-baseline.txt`). Its oracle was not edited.
- Duplicate broad pipeline/CLI runs were interrupted after overlapping the
  root's ongoing full pipeline run; interrupted logs are retained and are not
  presented as passing checks.

The path is size-only and intentionally narrow. It needs broader native
coverage, profile-bound generation and automatic candidate enumeration before
claiming generalized allocation benefit. Existing historical manifests remain
immutable. The new option is omitted when empty in JSON, so old default profile
IDs remain valid; any nonempty external table path is explicitly refused by PGO.
