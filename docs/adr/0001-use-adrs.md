# ADR-0001: Use Architecture Decision Records

## Status
Accepted

## Context
The MinZ project has grown significantly, with multiple contributors and evolving architecture. We need a way to:
- Document important technical decisions
- Understand the reasoning behind past choices
- Avoid revisiting the same discussions
- Onboard new contributors effectively

## Decision
We will use Architecture Decision Records (ADRs) to document all significant architectural decisions in the MinZ project. ADRs will be:
- Stored in the `/adr` directory
- Written in Markdown format
- Numbered sequentially (0001, 0002, etc.)
- Immutable once accepted (use new ADRs to supersede old ones)

## Consequences

### Positive
- Clear historical record of decisions
- Better onboarding for new contributors
- Reduced time spent on recurring discussions
- Improved project documentation

### Negative
- Additional documentation overhead
- Requires discipline to maintain

### Neutral
- Becomes part of the PR review process

## Alternatives Considered
- **Wiki pages**: Too disconnected from code
- **Code comments**: Not suitable for high-level decisions
- **Issue discussions**: Hard to find and not permanent

## References
- [Michael Nygard's ADR template](https://github.com/joelparkerhenderson/architecture_decision_record)
- [Thoughtworks Technology Radar on ADRs](https://www.thoughtworks.com/radar/techniques/lightweight-architecture-decision-records)