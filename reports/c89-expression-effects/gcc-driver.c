#include <stdio.h>
#include "source.c"
int main(void) {
 unsigned int a,b,v;
 for(a=0;a<=255;a++) for(b=0;b<=255;b++) {
 v=and_post(a,b);printf("and_post %u %u %u\n",a,b,v);
 v=or_post(a,b);printf("or_post %u %u %u\n",a,b,v);
 v=pick_post(a,b);printf("pick_post %u %u %u\n",a,b,v);
 v=effect_cond(a,b);printf("effect_cond %u %u %u\n",a,b,v);
 v=nested(a,b);printf("nested %u %u %u\n",a,b,v);
 }
 for(a=0;a<=255;a++) {
 v=trace_and(a);printf("trace_and %u %u %u\n",a,v,trace);
 v=trace_or(a);printf("trace_or %u %u %u\n",a,v,trace);
 v=trace_pick(a);printf("trace_pick %u %u %u\n",a,v,trace);
 v=loop_condition(a);printf("loop_condition %u %u 0\n",a,v);
 }
 return 0;
}
