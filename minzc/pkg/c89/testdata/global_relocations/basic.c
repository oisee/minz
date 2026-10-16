const char *message = "packet";
const char *suffix = "packet" + 2;
const char *table[3] = {"A", "B\n", "C"};
struct Record { unsigned char tag; const char *text; unsigned short value; };
struct Record record = {7, "xy", 0x1234};
unsigned char data[5] = {3, 5, 7, 11, 13};
unsigned char *middle = &data[2];
unsigned short *field = &record.value;
unsigned short callback(unsigned char);
unsigned short (*handler)(unsigned char) = callback;
unsigned short callback(unsigned char x) { return x + 17; }
unsigned short string_check(unsigned char x) {
 return (unsigned char)message[x % 7] + (unsigned char)suffix[x % 5] + (unsigned char)table[x % 3][0] + (unsigned char)record.text[x % 3];
}
unsigned short address_check(unsigned char x) {return middle[x % 3] + *field + handler(x);}
unsigned short static_check(unsigned char x) {
 static const char *text = "ab";
 static unsigned char value = 9;
 static unsigned char *p = &value;
 static unsigned short (*f)(unsigned char) = callback;
 return (unsigned char)text[x % 3] + *p + f(x);
}

struct Pair { unsigned char hi, lo; };
struct Pair make_pair(unsigned char x) { struct Pair p; p.hi = x; p.lo = x ^ 90; return p; }
struct Pair (*pair_handler)(unsigned char) = make_pair;
unsigned short consume_pair(struct Pair p) { return ((unsigned short)p.hi << 8) | p.lo; }
unsigned short (*pair_consumer)(struct Pair) = consume_pair;
unsigned short pair_check(unsigned char x) { struct Pair p = pair_handler(x); return pair_consumer(p); }
struct Nested { unsigned char pad; unsigned char values[3]; };
struct Nested nested = {99, {2, 4, 8}};
unsigned char *nested_pos = &nested.values[1];
unsigned char *backward = data + 4 - 2;
extern unsigned char forward_data[2];
unsigned char *forward_pos = &forward_data[1];
unsigned char forward_data[2] = {21, 34};
unsigned short offset_check(unsigned char x) { return nested_pos[x % 2] + backward[x % 3] + *forward_pos; }
