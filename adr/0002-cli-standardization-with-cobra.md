# ADR-0002: Standardize CLI Options with Cobra

## Status
Accepted

## Context
The MinZ toolchain consists of multiple command-line tools (mz, mza, mze, mzr), but they were using different libraries and conventions for handling command-line options:

- `mz` (compiler) used Cobra with proper short/long options
- `mza` (assembler) used Go's standard `flag` package with non-standard options like `-undoc`
- `mze` (emulator) used Go's `flag` package with duplicate options (`-t` and `-target` as separate flags)
- Inconsistent option naming: single-dash long options, missing short options, non-standard formats

This inconsistency led to:
- Poor user experience (unpredictable option formats)
- Maintenance burden (different patterns in each tool)
- Unprofessional appearance
- Difficulty in documentation

## Decision
Standardize all MinZ CLI tools to use the Cobra library (`github.com/spf13/cobra`) with strict adherence to Unix/POSIX conventions:

1. **All tools MUST use Cobra** - No direct use of Go's `flag` package
2. **Follow Unix conventions**:
   - Short options: single dash, single character (`-v`, `-o`)
   - Long options: double dash, kebab-case (`--verbose`, --output`)
   - Every common option should have BOTH forms
3. **Standard options across all tools**:
   - `-h, --help` for help
   - `-v, --verbose` for verbose output (or `--version` for version)
   - `-o, --output` for output file specification
4. **Consistent help text format** with clear sections

## Implementation
```go
// Standard pattern for all tools
var rootCmd = &cobra.Command{
    Use:   "tool [input]",
    Short: "Brief description",
    Long:  `Detailed multi-line description...`,
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        // Implementation
    },
}

func init() {
    // Always use VarP for short+long pairing
    rootCmd.Flags().StringVarP(&output, "output", "o", "", "output file")
    rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}
```

## Consequences

### Positive
- **Consistent UX**: Users can predict option formats across all tools
- **Professional appearance**: Follows industry standards (Git, Docker, Go, etc.)
- **Better help text**: Cobra automatically generates well-formatted help
- **Composability**: Short options can be combined (`-vc` = `-v -c`)
- **Maintenance**: Single pattern to follow for all tools
- **Documentation**: Easier to document consistent interfaces

### Negative
- **Migration effort**: Had to rewrite mza and mze main.go files
- **Dependency**: All tools now depend on Cobra library
- **Breaking change**: Users of old option formats need to update scripts

### Neutral
- File size slightly increases due to Cobra dependency
- Need to update documentation and examples

## Migration Path
1. ✅ Rewrite `mza` to use Cobra (completed)
2. ✅ Rewrite `mze` to use Cobra (completed)
3. ✅ Update CONTRIBUTING.md with CLI standards
4. ✅ Document in ADR

## Alternatives Considered

### Keep using Go's flag package
- **Pros**: No external dependency, simpler
- **Cons**: No automatic short/long pairing, no standard help generation, requires manual implementation of Unix conventions
- **Rejected because**: The benefits of consistency outweigh the cost of a dependency

### Use other CLI libraries (urfave/cli, kingpin)
- **Pros**: Also provide good CLI interfaces
- **Cons**: Less popular than Cobra, different API patterns
- **Rejected because**: Cobra is the de facto standard in Go ecosystem (used by Kubernetes, Docker, GitHub CLI)

### Custom wrapper around flag package
- **Pros**: No external dependency, tailored to our needs
- **Cons**: Maintenance burden, reinventing the wheel
- **Rejected because**: Cobra already solves this problem well

## References
- [POSIX Utility Conventions](https://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap12.html)
- [GNU Coding Standards - Command Line Interfaces](https://www.gnu.org/prep/standards/html_node/Command_002dLine-Interfaces.html)
- [Cobra Documentation](https://github.com/spf13/cobra)
- [The Art of Command Line](https://github.com/jlevy/the-art-of-command-line)

## Related ADRs
- None yet

## Date
2025-08-10