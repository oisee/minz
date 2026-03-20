/* MinZ C89 vs SDCC 4.2.0 — pure C benchmark */

/* 1. Constant folding through loops */
int fibonacci(int n) {
    int a = 0, b = 1;
    while (n > 0) { int t = a + b; a = b; b = t; n = n - 1; }
    return a;
}
int fib10(void) { return fibonacci(10); }

int sum_to(int n) {
    int total = 0, i = 0;
    while (i < n) { total = total + i; i = i + 1; }
    return total;
}
int sum10(void) { return sum_to(10); }

int factorial(int n) {
    int r = 1;
    while (n > 1) { r = r * n; n = n - 1; }
    return r;
}
int fact6(void) { return factorial(6); }

/* 2. Per-function ABI — no wasted EX DE,HL */
int twice(int x) { return x + x; }
int add(int a, int b) { return a + b; }

/* 3. Conditional return — RET cc instead of JP cc */
unsigned char abs_diff(unsigned char a, unsigned char b) {
    if (a > b) return a - b;
    return b - a;
}

unsigned char clamp(unsigned char val, unsigned char lo, unsigned char hi) {
    if (val < lo) return lo;
    if (val > hi) return hi;
    return val;
}

unsigned char min8(unsigned char a, unsigned char b) {
    if (a < b) return a;
    return b;
}

/* 4. Cascading fold — entire expression collapsed */
int square(int x) { return x * x; }
int demo(void) { return square(4) + twice(5); }

/* 5. Tail call optimization — CALL+RET → JP */
unsigned char wrap_min(unsigned char a, unsigned char b) { return min8(a, b); }
int wrap_twice(int x) { return twice(x); }
