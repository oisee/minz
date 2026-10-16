struct Trio { unsigned short a, b, c; };
struct Trio make_trio(unsigned char x) {
    struct Trio p = {0, 0, 0};
    p.a = x; p.b = x + 1; p.c = x + 2;
    return p;
}
unsigned short consume(struct Trio p) {
    return p.a + 3u * p.b + 5u * p.c;
}
unsigned short direct(unsigned char x) { return consume(make_trio(x)); }
unsigned short copied(unsigned char x) {
    struct Trio p = make_trio(x);
    return consume(p);
}
