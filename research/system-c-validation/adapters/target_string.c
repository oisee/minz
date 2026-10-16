/* Portable target runtime shim, separate from the upstream libraries. */
#include <string.h>
void *memcpy(void *dst, const void *src, size_t n) {
    unsigned char *d=dst; const unsigned char *s=src;
    while(n--) *d++=*s++;
    return dst;
}
void *memset(void *dst, int c, size_t n) {
    unsigned char *d=dst;
    while(n--) *d++=(unsigned char)c;
    return dst;
}
size_t strlen(const char *s) { size_t n=0; while(s[n]) n++; return n; }
char *strchr(const char *s, int c) {
    for (;;) { if (*s==(char)c) return (char*)s; if (!*s) return NULL; s++; }
}
