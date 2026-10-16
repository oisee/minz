#import "library.c"
struct Pair { unsigned char hi, lo; };
struct Pair make_pair(unsigned char);
unsigned short consume(struct Pair);
unsigned short check(unsigned char x) {
    struct Pair p=make_pair(x);
    return consume(p);
}
