# GPU Backend Notes

## Verified Execution (2026-03-29)

| Backend | Compiles | Executes | Hardware |
|---------|:---:|:---:|----------|
| **CUDA** | ✓ | ✓ | NVIDIA RTX 4060 Ti, CUDA 12.9 |
| **OpenCL** | ✓ | ✓ | NVIDIA via OpenCL 3.0 |
| **Vulkan GLSL** | ✓ (single func) | ✗ (needs runner) | Vulkan 1.3.275 available |
| **Metal** | ✓ | ✓ **verified** | Apple M2 (via SSH to m2) |

## CUDA — Fully Working

```bash
# Generate + compile + run
mz program.nanz --emit cuda -o /tmp/prog.cu
nvcc /tmp/prog.cu harness.cu -o /tmp/prog
/tmp/prog
```

**Design:** Each MinZ function → `__global__` kernel. First param = `tid` (thread ID). Data-parallel: 256 threads execute same function with different first arg. Results written to `results[tid]`.

**Verified:** `add(tid,0)`, `square(tid)`, `double(tid)` — all 256 values correct including u8 overflow.

## OpenCL — Fully Working

```bash
# Compile host + kernel at runtime
gcc host.c -lOpenCL -o prog
./prog  # loads kernel source, builds, runs
```

Same data-parallel model as CUDA. Kernel string compiled at runtime via `clCreateProgramWithSource`.

## Vulkan GLSL — Compiles to SPIR-V (with fixes)

```bash
# Generate GLSL compute shader
mz program.nanz --emit vulkan -o /tmp/prog.comp

# Compile to SPIR-V
glslangValidator -V prog.comp -o prog.spv
```

**Known issues in mir2gpu Vulkan output:**
1. Comment line before `#version 450` — glslangValidator rejects this
2. Non-main functions declared as `uint func()` with `return;` (void return from uint)
3. Multiple entry points — Vulkan allows only one `main()`

**Fix needed:** For multi-function programs, inline helper functions into `main()` or emit them as regular GLSL functions (not entry points).

**Single-function programs work** after removing the leading comment.

**To run Vulkan compute:** Need Vulkan compute dispatch code (~100 LOC C with VkInstance, VkDevice, VkComputePipeline, VkCommandBuffer). Available on this machine (Vulkan 1.3.275 + NVIDIA ICD).

## Metal — Syntax Only

```bash
mz program.nanz --emit metal -o /tmp/prog.metal
```

Generates valid Metal Shading Language with `kernel void`, `device uint*`, `thread_position_in_grid`.

**Verified on Apple M2 (macOS 15.5) via SSH:**
```bash
# On i7 (Linux):
mz program.nanz --emit metal -o /tmp/minz.metal
scp /tmp/minz.metal m2:/tmp/

# On m2 (macOS):
xcrun -sdk macosx metal -c /tmp/minz.metal -o /tmp/minz.air
xcrun -sdk macosx metallib /tmp/minz.air -o /tmp/minz.metallib
swiftc runner.swift -framework Metal -framework Foundation -o runner
./runner
# → add: PASS (256 threads)
# → square: PASS (256 threads)
# → fn_double: PASS (256 threads)
# → fn_main: PASS (256 threads)
```

All 4 kernels verified: add, square, double, main. 256 threads each, u8 overflow correct.

## GPU Codegen Design (mir2gpu)

```
MIR2 Module
  ↓
For each function:
  - Emit as __global__ kernel (CUDA) / __kernel (OpenCL) / void main() (Vulkan) / kernel void (Metal)
  - First MIR2 param → tid (thread index)
  - Remaining params → initialized to 0 (data-parallel sweep)
  - All arithmetic → & 0xFF mask (u8 semantics)
  - results[tid] = return value
```

**Limitation:** Functions are independent kernels, no inter-function calls. Recursive functions (fibonacci) constant-fold at MIR2 level before GPU emission.

## Action Items

1. **Vulkan fix:** Remove comment before `#version`, inline helpers into main
2. **CUDA runner in Go:** Auto-generate harness, compile via nvcc, execute
3. **OpenCL runner in Go:** Use Go OpenCL bindings for automated testing
4. **Vulkan compute runner:** Minimal Vulkan dispatch (~100 LOC)
5. **Multi-function support:** Emit helper functions as `__device__` (CUDA) / regular functions (GLSL)
