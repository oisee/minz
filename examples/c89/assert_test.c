/* C89 frontend: assert test */

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
