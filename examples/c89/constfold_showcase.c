// Functions that MinZ can fully evaluate at compile time
// SDCC generates runtime code; MinZ folds to constants

int fibonacci(int n) {
    int a = 0, b = 1;
    while (n > 0) {
        int t = a + b;
        a = b;
        b = t;
        n = n - 1;
    }
    return a;
}

// fib(10) = 55 — MinZ evaluates at compile time
int fib10(void) { return fibonacci(10); }

// Sum 1..n via loop
int sum_to(int n) {
    int total = 0;
    int i = 0;
    while (i < n) {
        total = total + i;
        i = i + 1;
    }
    return total;
}

// sum_to(10) = 45 — compile-time constant
int sum10(void) { return sum_to(10); }

// Factorial
int factorial(int n) {
    int result = 1;
    while (n > 1) {
        result = result * n;
        n = n - 1;
    }
    return result;
}

// fact(6) = 720 — compile-time constant
int fact6(void) { return factorial(6); }
