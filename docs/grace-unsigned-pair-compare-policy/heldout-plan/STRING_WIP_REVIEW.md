# String repair WIP review

Read `b989352c` in the permanent `string-representation` checkout. Five production paths are limited to parser literal dispatch/bounds, ordinary counted-print lowering, VM representation storage, and native literal validation/empty-data syntax. No CPU execution or source edit by this reviewer.

VM C/S/L storage preserves cache identity and representation byte lengths; parser and native emitter reject oversize S/L strings. Empty data declarations no longer contain a trailing comma. Counted output evaluates its argument once, loads the stored prefix count, advances to data, prints exactly the count through ordinary MIR loads/calls, and restores lexical bindings on exit. Opaque pointers retain CString behavior.

Concrete issue sent to the owner: synthetic pointer/count names use `l.fresh`, which only increments a sequence. Ordinary assignment lowering resolves a module global before a local binding. A legal source global matching a generated name (for example `__print_ptr_1`) can therefore receive the synthetic pointer update, while the local pointer fails to advance. Reserve these generated names against module globals/functions and live local/alias names, and add a meaningful source-global collision fixture with unchanged global value and exact print output. Do not broaden global assignment semantics just to repair this new path.

Final source freeze and actual automatic-allocation fixtures remain pending. Test non-tail print continuation, live values across output, embedded NUL, empty formats, neighboring literals, and byte-count boundaries. No generic peephole change is justified by a tail `RET Z` alone. Maximum representable L length is separate from whether a particular 64 KiB target can place the entire encoded object.
