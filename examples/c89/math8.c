/* C89 corpus: 8-bit math (unsigned char arithmetic) */

unsigned char min8(unsigned char a, unsigned char b) {
    if (a < b) return a;
    return b;
}

unsigned char max8(unsigned char a, unsigned char b) {
    if (a > b) return a;
    return b;
}

unsigned char abs_diff(unsigned char a, unsigned char b) {
    if (a > b) return a - b;
    return b - a;
}

unsigned char clamp8(unsigned char val, unsigned char lo, unsigned char hi) {
    if (val < lo) return lo;
    if (val > hi) return hi;
    return val;
}

unsigned char saturate_add(unsigned char a, unsigned char b) {
    int sum = a + b;
    if (sum > 255) return 255;
    return sum;
}

// assert min8(3, 7) == 3 via mir2
// assert min8(7, 3) == 3 via mir2
// assert max8(3, 7) == 7 via mir2
// assert abs_diff(10, 3) == 7 via mir2
// assert abs_diff(3, 10) == 7 via mir2
// assert abs_diff(5, 5) == 0 via mir2
// assert clamp8(50, 10, 100) == 50 via mir2
// assert clamp8(5, 10, 100) == 10 via mir2
// assert clamp8(200, 10, 100) == 100 via mir2
// assert saturate_add(100, 100) == 200 via mir2
// assert saturate_add(200, 200) == 255 via mir2
