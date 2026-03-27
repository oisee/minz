# mir2gpu Testing Guide

## Verified Backends

| Backend | GPU | Machine | Status | Command |
|---------|-----|---------|--------|---------|
| CUDA | NVIDIA (any) | i7 | **256/256** ✅ | `go test -run TestCUDA_RealGPU` |
| OpenCL | NVIDIA (any) | i7 | **256/256** ✅ | `go test -run TestOpenCL_RealGPU` |
| Vulkan | AMD RX 580 | i3 | **256/256** ✅ | Manual (see below) |
| Metal | Apple M2 | Mac | **Pending** ⏳ | Manual (see below) |

## Automatic Tests (i7 with NVIDIA)

```bash
cd minzc
go test ./pkg/mir2gpu/ -v -timeout 60s
```

Runs all 7 tests:
- 4 codegen (generate source for all backends)
- CUDA 256/256 on real GPU (needs nvcc + NVIDIA)
- OpenCL 256/256 on real GPU (needs libOpenCL + GPU)
- Vulkan SPIR-V compile (needs glslangValidator)

## Vulkan on i3 (AMD RX 580)

### Step 1: Copy shader to i3
```bash
scp /tmp/minz_vulkan_test.comp i3:~/
```

### Step 2: Compile GLSL → SPIR-V on i3
```bash
ssh i3
glslangValidator -V minz_vulkan_test.comp -o minz_vulkan_test.spv
```

### Step 3: Run with Vulkan dispatch
```bash
# Use z80-optimizer's vulkan_dispatch.c (120 lines):
cd ~/dev/z80-optimizer/cuda
gcc vulkan_dispatch.c -o vk_test -lvulkan -lm
./vk_test ../minz_vulkan_test.spv 256
# Expected: double(0)=0, double(1)=2, ... Correct: 256/256
```

## Metal on M2 Mac

### Step 1: Copy files to Mac
```bash
scp /tmp/minz_metal_test.metal mac:~/
scp /tmp/minz_metal_host.swift mac:~/
```

### Step 2: Compile and run on Mac
```bash
ssh mac
# The Swift host embeds the Metal shader source, so just compile the Swift:
swiftc -framework Metal -framework Foundation minz_metal_host.swift -o metal_test
./metal_test
# Expected: double(0)=0, double(1)=2, ... Correct: 256/256
```

### Alternative: compile .metal separately
```bash
xcrun metal -c minz_metal_test.metal -o minz_metal_test.air
xcrun metallib minz_metal_test.air -o minz_metal_test.metallib
# Then load .metallib in Swift host instead of source compilation
```

## Adding New Test Functions

1. Write Nanz source: `fun triple(x: u8) -> u8 { return x + x + x }`
2. Generate: `mir2gpu.StandaloneCUDA(m, "triple", 256)`
3. Verify: update `verifyExpr()` in standalone_cuda.go: `"(i * 3) & 0xFF"`
4. Run: all backends automatically get the new function

## Architecture

```
Nanz source → HIR → MIR2 → mir2gpu universal body → backend wrap
                                    │
                    ┌───────────────┼───────────────┬──────────────┐
                    ▼               ▼               ▼              ▼
              CUDA (.cu)     OpenCL (.cl)    Vulkan (.comp)   Metal (.metal)
              nvcc           gcc -lOpenCL    glslangValidator  xcrun metal
              NVIDIA GPU     any OpenCL      any Vulkan        Apple GPU
```

95% of code is universal. 5% is backend-specific (kernel qualifier, thread ID, params).
