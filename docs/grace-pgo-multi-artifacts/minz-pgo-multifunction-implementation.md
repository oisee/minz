Implemented multi-function selection with one strict profile load, unique function models, and exactly one decision per site. Uncovered functions retain original with empty totals. The single-model wrapper preserves legacy attribution.

`ProfileSelection.Models` is mutually exclusive with `Model`; every model’s compilation ID is checked before mutation. The entire selection request remains excluded from compilation IDs.

Tests passed using the specified Go toolchain/cache:

- HIR selection and refusal tests.
- Pipeline two-function VM/Z80 integration: short original, long pointer-walk; 210 and 3,749 T-states.
- Profile and measurement regression tests.

Limits: integration uses fixture coefficients, not newly generated measurement evidence. Concurrent edits were preserved, including the guard requiring joint evidence for multiple pointer-walk choices. No commits or measurement/CLI edits by me.