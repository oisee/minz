Implemented in two new `pkg/pipeline` files.

```go
func MeasureLoopCostModel(
    factory func() (*hir.Module, error),
    function string,
    workload hir.ProfileWorkload,
    options Options,
) (hir.LoopCostModel, *LoopCostEvidence, error)
```

Validates all 1,022 cases, derives coefficients from execution, rejects nonlinear timings, and binds evidence to compatibility, configuration, and assembled output.

Signature saved to `/tmp/minz-pgo-model-api.txt`.

Full pipeline and HIR tests passed using the requested Go toolchain and cache. No commit created.