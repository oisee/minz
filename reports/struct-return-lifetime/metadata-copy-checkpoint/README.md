Follow-up: chained pointer struct literal assignments now use recursive inline-object copies for nested struct members, including nonliteral member values. Extended unchanged nested oracle checks nested_nonliteral over27 native calls in three modes, plus previous153 cases; no address payload stored into inline struct layout.

Call ABI metadata cloning now owns descriptor slices and mutable nested struct/array/tuple/range/pointer type graphs, while retaining canonical scalar singleton identity. A memo preserves recursive graphs and internal alias relationships. HIR renamed indirect signatures clone rather than share backing metadata; MIR/VIR instruction clone helpers own call metadata. TestCallABIMetadataCloneOwnsNestedTypes mutates copied names, fields, parameter slices and buffer index and verifies original unchanged, including recursive pointer types.

This is a source/metadata correctness follow-up, not a claim of JSON type roundtrip or complete aggregate profile entry support.
