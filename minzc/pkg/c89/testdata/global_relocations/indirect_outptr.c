struct Pair { unsigned char hi, lo; };

void fill_pair(unsigned char x, struct Pair *p) { p->hi = x; p->lo = x ^ 90; }
void (*pair_filler)(unsigned char, struct Pair *) = fill_pair;
unsigned short outptr_check(unsigned char x) { struct Pair p; pair_filler(x, &p); return ((unsigned short)p.hi << 8) | p.lo; }
