/*
 * C99 Control flow — thorough branch/loop testing on Z80
 *
 * mzv: echo "" | mzv -H c99_control_flow.c
 */
#include <stdint.h>
#include <stdbool.h>

/* if/else chains */
uint8_t classify(uint8_t x) {
    if (x == 0) return 0;
    else if (x < 10) return 1;
    else if (x < 100) return 2;
    else return 3;
}

/* switch (desugared to if/else) */
uint8_t day_type(uint8_t day) {
    switch (day) {
    case 0: case 6: return 1;  /* weekend */
    case 1: case 2: case 3: case 4: case 5: return 2;  /* weekday */
    default: return 0;
    }
}

/* while loop */
uint8_t count_down(uint8_t n) {
    uint8_t sum = 0;
    while (n > 0) {
        sum += n;
        n--;
    }
    return sum;
}

/* for loop with inline decl (C99) */
uint8_t factorial(uint8_t n) {
    uint8_t result = 1;
    for (int i = 1; i <= n; i++) {
        result *= i;
    }
    return result;
}

/* do-while */
uint8_t digits(uint8_t n) {
    uint8_t count = 0;
    do {
        count++;
        n /= 10;
    } while (n > 0);
    return count;
}

/* nested loops */
uint8_t mul_table_sum(uint8_t n) {
    uint8_t sum = 0;
    for (int i = 1; i <= n; i++) {
        for (int j = 1; j <= n; j++) {
            sum += 1;
        }
    }
    return sum;
}

/* early return */
uint8_t find_first_even(uint8_t a, uint8_t b, uint8_t c) {
    if ((a & 1) == 0) return a;
    if ((b & 1) == 0) return b;
    if ((c & 1) == 0) return c;
    return 0;
}

/* ternary chain */
uint8_t sign(int16_t x) {
    return x > 0 ? 1 : x < 0 ? 2 : 0;
}

/* break in loop */
uint8_t first_divisor(uint8_t n) {
    for (int i = 2; i < n; i++) {
        if (n % i == 0) return i;
    }
    return n;
}

/* goto */
uint8_t test_goto(uint8_t x) {
    if (x > 10) goto big;
    return 0;
big:
    return 1;
}

// assert classify(0) == 0 via mir2
// assert classify(5) == 1 via mir2
// assert classify(50) == 2 via mir2
// assert classify(200) == 3 via mir2
// switch fallthrough: desugared to if/else, multi-case needs work
// TODO assert day_type(0) == 1 via z80
// TODO assert day_type(3) == 2 via z80
// assert count_down(5) == 15 via mir2
// assert count_down(0) == 0 via mir2
// assert factorial(5) == 120 via mir2
// assert factorial(1) == 1 via mir2
// assert digits(0) == 1 via mir2
// assert digits(9) == 1 via mir2
// assert digits(99) == 2 via mir2
// assert digits(100) == 3 via mir2
// assert mul_table_sum(3) == 9 via mir2
// assert find_first_even(3, 5, 8) == 8 via mir2
// assert find_first_even(2, 5, 8) == 2 via mir2
// assert sign(42) == 1 via mir2
// assert sign(0) == 0 via mir2
// assert first_divisor(12) == 2 via mir2
// assert first_divisor(7) == 7 via mir2
// assert test_goto(5) == 0 via mir2
// assert test_goto(20) == 1 via mir2
