/* C89: a returned aggregate can be passed by value to another function. */
struct Pair { unsigned char hi, lo; };
struct Pair make_pair(unsigned char x) {
    struct Pair p = { 0, 0 };
    p.hi = x;
    p.lo = (unsigned char)(x + 1);
    return p;
}
unsigned int consume(struct Pair p) {
    return (unsigned int)p.hi * 256 + p.lo;
}
unsigned int check(unsigned char x) {
    struct Pair p = make_pair(x);
    return consume(p);
}
