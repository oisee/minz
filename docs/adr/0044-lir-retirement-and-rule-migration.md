# ADR-0044: Deprecate native LIR; migrate useful transforms to HIR/MIR

**Status:** Accepted; migration queued, not implemented
**Date:** 2026-10-10

## Decision

Native `pkg/lir` code generation is deprecated. Production remains the shared
HIR → MIR2 → PBQP/codegen path. Native LIR emission is already disabled;
this decision does not enable it or introduce fallback between backends.

Keep LIR source, focused tests and explicit research/convergence entry points
as references during migration. They are experiments, not authoritative correctness
oracles. Preserve CLI compatibility: `--lir` continues to report disabled native
emission and emits PBQP output.

Inventory all useful LIR behavior, compare it with the main path, and transfer
missing benefits one at a time. Use Grace for HIR/MIR rewrites where suitable;
target instruction selection and allocation constraints belong in the target
backend/cost model. Migrating a rule does not require retaining the ISLE DSL.
Do not rebuild the entire LIR pipeline inside Grace or replace PBQP implicitly.

## Acceptance

Each transfer needs a concrete source case, independent expected behavior and
VM differential checks where representable. Target changes also need assembly
validation and target execution. Measure compiler cost, code size and/or target
runtime against production. Deduplicate behavior already present in production.
The small HIR Grace experiment does not establish corpus-wide benefits or LIR
equivalence.

## Execution plan

[Migration backlog](../lir-retirement-backlog.md).
[Measured HIR/MIR Grace experiment](../grace-hir-vm-e2e.md).
Delete components only after their useful behavior/tests have a destination or
an explicit decision that the behavior is redundant or unsupported.
