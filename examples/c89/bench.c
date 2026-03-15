/* MinZ vs SDCC benchmark — small Z80-relevant functions */

int twice(int x) {
    return x + x;
}

int add(int a, int b) {
    return a + b;
}

int max(int a, int b) {
    if (a > b) return a;
    return b;
}

unsigned char abs_diff(unsigned char a, unsigned char b) {
    if (a > b) return a - b;
    return b - a;
}

int sum_to(int n) {
    int total = 0;
    int i = 0;
    while (i < n) {
        total = total + i;
        i = i + 1;
    }
    return total;
}

unsigned char clamp8(unsigned char val, unsigned char lo, unsigned char hi) {
    if (val < lo) return lo;
    if (val > hi) return hi;
    return val;
}
