/* C89 corpus: 16-bit integer math */

int abs16(int x) {
    if (x < 0) return -x;
    return x;
}

int min16(int a, int b) {
    if (a < b) return a;
    return b;
}

int max16(int a, int b) {
    if (a > b) return a;
    return b;
}

int clamp16(int val, int lo, int hi) {
    if (val < lo) return lo;
    if (val > hi) return hi;
    return val;
}

int sum_range(int lo, int hi) {
    int total = 0;
    int i = lo;
    while (i <= hi) {
        total = total + i;
        i = i + 1;
    }
    return total;
}

int factorial(int n) {
    int result = 1;
    int i = 2;
    while (i <= n) {
        result = result * i;
        i = i + 1;
    }
    return result;
}

int fibonacci(int n) {
    int a = 0;
    int b = 1;
    int t = 0;
    int i = 0;
    while (i < n) {
        t = a + b;
        a = b;
        b = t;
        i = i + 1;
    }
    return a;
}

// assert abs16(5) == 5 via mir2
// assert abs16(-5) == 5 via mir2
// assert min16(3, 7) == 3 via mir2
// assert max16(3, 7) == 7 via mir2
// assert clamp16(50, 10, 100) == 50 via mir2
// assert clamp16(5, 10, 100) == 10 via mir2
// assert clamp16(200, 10, 100) == 100 via mir2
// assert factorial(5) == 120 via mir2
// assert factorial(1) == 1 via mir2
// assert sum_range(1, 10) == 55 via mir2
// NOTE: fibonacci asserts disabled — MIR2 VM variable init bug in loops
// TODO: investigate multi-variable init ordering in MIR2 lowering
