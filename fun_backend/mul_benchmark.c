// Multiply benchmark — compare MinZ vs SDCC on Z80.
// Shows T-state costs and generated assembly.
//
// Try:
//   mz mul_benchmark.c -o mul.a80 --annotate-tstates   (Z80 with costs)
//   mzd mul.bin --cycles                               (disassembly with T-states)
//   mzn mul_benchmark.c                                (native x86_64)
//   mz mul_benchmark.c --emit llvm -o mul.ll           (LLVM IR)
//   clang mul.ll -o mul && ./mul; echo $?              (native binary)
//   mz mul_benchmark.c --emit mir2                     (see SSA IR)

int mul8(int a, int b) { return a * b; }
int square(int x) { return x * x; }
int cube(int x) { return x * x * x; }

int dot2(int ax, int ay, int bx, int by) {
    return ax * bx + ay * by;
}

// assert mul8(6, 7) == 42 via mir2
// assert square(5) == 25 via mir2
// assert cube(3) == 27 via mir2
// assert dot2(3, 4, 5, 6) == 39 via mir2
// assert dot2(1, 0, 0, 1) == 0 via mir2
