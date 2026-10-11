#include <stdio.h>
#include "source.c"
int main(void) { unsigned int n; for(n=0;n<=32;n++) {
 state=99; {unsigned int v=while_post(n); printf("while_post %u %u %u\n",n,v,state); }
 state=99; {unsigned int v=do_post(n);printf("do_post %u %u %u\n",n,v,state);}
 state=99; {unsigned int v=for_post(n);printf("for_post %u %u %u\n",n,v,state);}
 } return 0; }
