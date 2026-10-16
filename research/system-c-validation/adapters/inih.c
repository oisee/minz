/* Frozen upstream source included unchanged. Configuration: stack line buffer. */
#define INI_USE_STACK 1
#define INI_MAX_LINE 128
#define INI_STOP_ON_FIRST_ERROR 1
#include "../upstream/inih/ini.c"

/* Explicit unavailable-file stubs; only the string API is validated. */
FILE *fopen(const char *name, const char *mode) { return NULL; }
int fclose(FILE *file) { return -1; }
char *fgets(char *line, int size, FILE *file) { return NULL; }

unsigned char input[256];
unsigned char output[256];
unsigned int result;
static unsigned int used;
static void append(const char *p) {
    while (*p && used < 255) output[used++] = *p++;
}
static int record(void *user, const char *section, const char *name, const char *value) {
    append(section); append("/"); append(name); append("="); append(value); append(";");
    return 1;
}
void execute(void) {
    unsigned int i;
    for (i=0;i<256;i++) output[i]=0;
    used=0;
    result=ini_parse_string((const char*)input, record, NULL);
}

#include "target_string.c"
