/* C89 frontend test: minimal function */

int twice(int x) {
    return x + x;
}

int add(int a, int b) {
    return a + b;
}

int max(int a, int b) {
    if (a > b)
        return a;
    return b;
}

unsigned char abs_diff(unsigned char a, unsigned char b) {
    if (a > b)
        return a - b;
    return b - a;
}
