#include <stdio.h>
#include "source.c"
int main(void) {
 unsigned int n, v;
 for(n=0;n<=32;n++) {
  state=99;v=while_post(n);printf("while_post %u %u %u\n",n,v,state);
  state=99;v=do_post(n);printf("do_post %u %u %u\n",n,v,state);
  state=99;v=for_post(n);printf("for_post %u %u %u\n",n,v,state);
  state=99;v=for_continue(n);printf("for_continue %u %u %u\n",n,v,state);
  state=99;v=do_continue(n);printf("do_continue %u %u %u\n",n,v,state);
  state=99;v=while_call(n);printf("while_call %u %u %u\n",n,v,state);
 }
 for(n=0;n<=255;n++) {
  state=99;v=choose(n);printf("choose %u %u %u\n",n,v,state);
  state=99;v=choose_else(n);printf("choose_else %u %u %u\n",n,v,state);
 }
 return 0;
}
