# ADR-0005: Future Consideration - Monorepo Structure

## Status
Draft

## Context
The MinZ project currently has a mixed structure:
- Tree-sitter grammar at the root (`grammar.js`, `package.json`)
- Go compiler in `/minzc` subdirectory
- Examples scattered across multiple directories
- Documentation in various locations

This structure has evolved organically but presents challenges:
- Unclear where to find specific components
- Mixed language dependencies at root level
- Difficult to manage tool-specific dependencies
- CI/CD complexity

## Decision (Proposed)
*This is a future consideration, not yet implemented.*

Consider restructuring the repository as a proper monorepo:

```
minz/
├── grammar/          # Tree-sitter grammar
│   ├── package.json
│   └── grammar.js
├── compiler/         # Go compiler (currently minzc/)
│   ├── go.mod
│   └── cmd/
├── tools/           # Additional tools
│   ├── vscode-extension/
│   └── assembler/
├── examples/        # All examples in one place
├── docs/           # All documentation
├── adr/            # Architecture decisions
└── stdlib/         # Standard library
```

## Consequences

### Positive
- **Clear organization**: Each component in its logical place
- **Independent versioning**: Tools can version independently
- **Better CI/CD**: Can test/build only changed components
- **Cleaner dependencies**: No mixed package.json/go.mod at root

### Negative
- **Migration effort**: Breaking change for existing users
- **Build complexity**: Need monorepo tooling (Bazel, Nx, etc.)
- **Import paths change**: All Go imports need updating

### Neutral
- Popular projects use this structure (Kubernetes, React)
- Requires documentation update

## Alternatives Considered

### Keep current structure
- **Pros**: No migration needed
- **Cons**: Continuing confusion and complexity

### Separate repositories
- **Pros**: Complete independence
- **Cons**: Harder to coordinate changes, versioning nightmares

## When to Implement
Consider implementing when:
- [ ] Project reaches v1.0 stability
- [ ] Multiple language targets are added
- [ ] Team grows beyond 5 contributors
- [ ] CI/CD becomes unmanageable

## References
- [Monorepo vs Polyrepo](https://earthly.dev/blog/monorepo-vs-polyrepo/)
- [Google's Monorepo](https://cacm.acm.org/magazines/2016/7/204032-why-google-stores-billions-of-lines-of-code-in-a-single-repository/fulltext)
- [Lerna](https://lerna.js.org/) - JS monorepo tool
- [Bazel](https://bazel.build/) - Google's build system

## Related ADRs
- Future: ADR-00XX: Build System Selection
- Future: ADR-00XX: Versioning Strategy

## Date
2025-08-10 (Draft)