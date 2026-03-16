// Static local variables test

// Static local retains value across calls
int counter(void) {
    static int n = 0;
    n = n + 1;
    return n;
}

int test_counter(void) {
    counter();      // n becomes 1
    counter();      // n becomes 2
    return counter(); // n becomes 3
}
// assert test_counter() == 3 via mir2

// Two functions with same-name static locals — independent
int counterA(void) {
    static int n = 0;
    n = n + 10;
    return n;
}

int counterB(void) {
    static int n = 0;
    n = n + 1;
    return n;
}

int test_independent(void) {
    counterA();  // A.n = 10
    counterB();  // B.n = 1
    counterB();  // B.n = 2
    return counterA(); // A.n = 20
}
// assert test_independent() == 20 via mir2
