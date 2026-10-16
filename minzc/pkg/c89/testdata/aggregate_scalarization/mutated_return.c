/* C89: the returned value includes changes made after initialization. */
struct Pair { unsigned char hi, lo; };
struct Pair make_pair(unsigned char x) {
    struct Pair p = { 0, 0 };
    p.hi = x;
    p.lo = (unsigned char)(x + 1);
    p.hi = (unsigned char)(p.hi + 2);
    return p;
}
unsigned int check(unsigned char x) {
    struct Pair p = make_pair(x);
    return (unsigned int)p.hi * 256 + p.lo;
}
