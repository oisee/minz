/* Host reference: same adapters/configuration and upstream bytes. */
#include <stdio.h>
#ifdef VALIDATE_INIH
#include "adapters/inih.c"
#else
#include "adapters/heatshrink.c"
#endif
int main(void) {
    if (fread(input,1,sizeof(input),stdin)!=sizeof(input)) return 2;
#ifndef VALIDATE_INIH
    unsigned char n[2];
    if (fread(n,1,2,stdin)!=2) return 2;
    input_length=n[0]+256u*n[1];
#endif
    execute();
    unsigned char r[2]={(unsigned char)result,(unsigned char)(result>>8)};
    if (fwrite(r,1,2,stdout)!=2 || fwrite(output,1,sizeof(output),stdout)!=sizeof(output)) return 3;
    return 0;
}
