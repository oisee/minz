# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for the MinZ project.

## What is an ADR?

An Architecture Decision Record captures an important architectural decision made along with its context and consequences.

## ADR Format

We use a lightweight format inspired by Michael Nygard's template:

- **Title**: ADR-NNNN: Short descriptive title
- **Status**: Draft | Proposed | Accepted | Deprecated | Superseded
- **Context**: What is the issue we're seeing that motivates this decision?
- **Decision**: What is the change that we're proposing/doing?
- **Consequences**: What becomes easier or harder because of this change?

## Index

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| [ADR-0001](0001-use-adrs.md) | Use Architecture Decision Records | Accepted | 2025-08-10 |
| [ADR-0002](0002-cli-standardization-with-cobra.md) | Standardize CLI Options with Cobra | Accepted | 2025-08-10 |
| [ADR-0003](0003-platform-independent-compilation.md) | Platform-Independent Compilation for Z80 Systems | Accepted | 2025-08-09 |
| [ADR-0004](0004-character-literals-in-assembly.md) | Character Literals in Assembly | Accepted | 2025-08-09 |
| [ADR-0005](0005-future-consideration-monorepo-structure.md) | Future Consideration - Monorepo Structure | Draft | 2025-08-10 |

## Creating a New ADR

1. Copy the template: `cp template.md NNNN-title-of-decision.md`
2. Fill in the details
3. Submit for review via PR
4. Update this index when accepted

## Tooling

For managing ADRs at scale, consider:
- [adr-tools](https://github.com/npryce/adr-tools)
- [log4brains](https://github.com/thomvaill/log4brains)