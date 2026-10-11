# Typed call-spill certificate plumbing

The caller-save repair introduced EX (SP),HL around compiler-owned spill homes. The existing closed runner correctly refused its unproved stack access. This stage adds a distinct typed finite save/restore descriptor rooted in actual MIR, allocation, assembled call target, spill symbol and exact instruction bytes. Runtime admission checks initialized HL slots, call depth and identity, saved-word integrity and LIFO retirement. No numeric stack-address allowance is added.

The target captures all resolved stages and their canonical typed SHA. Policy associations bind that hash; invocation setup and successful completion require no outstanding stages. Existing result-frame recipes may not overlap call-spill instruction PCs.

Owner controls include a positive permission preflight and ten invalid activation states refusing before RAM changes. Compile-only captures preserve source, assembly, binary, allocation, access origins and typed stages. Two bounded CLI collect/replay checks are running under the retained 4d818 executable; their full temporary directories are exported before cleanup. No final native completion is claimed until terminal logs and independent saved audits are available.
