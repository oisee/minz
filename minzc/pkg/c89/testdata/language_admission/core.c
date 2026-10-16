#define FEATURE 2
#define JOIN_INNER(a,b) a##b
#define JOIN(a,b) JOIN_INNER(a,b)
#define STRING_INNER(x) #x
#define STRING(x) STRING_INNER(x)
#if defined(FEATURE) && FEATURE == 2
#define SELECTED 19
#else
#define SELECTED 99
#endif
unsigned char calls;
int touch(void) { calls++; return 7; }
int oldstyle(x)
unsigned char x;
{ return x + 17; }
implicit(x)
int x;
{ return x + 23; }
int before();
int forward(unsigned char x) { return before(x); }
int before(x)
int x;
{ return x + 31; }
unsigned short knr(unsigned char x) {return oldstyle(x) + implicit(x) + forward(x);}
unsigned short unevaluated(unsigned char x) {
    calls = 0;
    return sizeof(x++) + sizeof(touch()) + sizeof(unsigned long) + calls;
}
unsigned short preprocessing(unsigned char x) {
    unsigned short JOIN(value_, 2) = SELECTED;
    const char *s = STRING(packet);
    return value_2 + s[x % 6];
}
unsigned short stringbytes(unsigned char x) {
    const char *p = "A\n" "\101\0Z";
    const char *q = "A\n\101\0Z";
    return (unsigned char)p[x % 6] + (unsigned char)q[x % 6];
}
