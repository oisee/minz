/* C89 corpus: byte-array/string helpers (pointer ops) */

int my_strlen(unsigned char *s) {
    int len = 0;
    while (*s != 0) {
        len = len + 1;
        s = s + 1;
    }
    return len;
}

void my_memset(unsigned char *dst, unsigned char val, int n) {
    int i = 0;
    while (i < n) {
        *dst = val;
        dst = dst + 1;
        i = i + 1;
    }
}

void my_memcpy(unsigned char *dst, unsigned char *src, int n) {
    int i = 0;
    while (i < n) {
        *dst = *src;
        dst = dst + 1;
        src = src + 1;
        i = i + 1;
    }
}

int my_memcmp(unsigned char *a, unsigned char *b, int n) {
    int i = 0;
    while (i < n) {
        if (*a != *b) {
            if (*a < *b) return -1;
            return 1;
        }
        a = a + 1;
        b = b + 1;
        i = i + 1;
    }
    return 0;
}
