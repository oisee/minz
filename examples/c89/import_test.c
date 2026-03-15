/* Cross-file #import test */

#import "imported_math.c"

unsigned char sum_of_squares(unsigned char a, unsigned char b) {
    return square(a) + square(b);
}

unsigned char clamped(unsigned char x, unsigned char lo, unsigned char hi) {
    return min2(max2(x, lo), hi);
}

// assert sum_of_squares(3, 4) == 25 via mir2
// assert sum_of_squares(0, 5) == 25 via mir2
// assert clamped(50, 10, 100) == 50 via mir2
// assert clamped(5, 10, 100) == 10 via mir2
// assert clamped(200, 10, 100) == 100 via mir2
// assert cube(3) == 27 via mir2
// assert cube(5) == 125 via mir2
