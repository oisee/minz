unsigned char input, count;
unsigned char trace[32];
unsigned short result;
void mark(unsigned char v) { trace[count++] = v; }
unsigned short loops(unsigned char x) {
 unsigned char i=0, limit=x&7;
 unsigned short v=0;
 for (;i<limit;i++) { if(i==1) continue; if(i==4) break; mark(i+1); v=v*3+i; }
 i=0;
 while(i<(x&3)) { i++; mark(i+8); v+=i; }
 i=0;
 do { i++; mark(i+16); if(i==1) continue; v+=i; } while(i<(x&3));
 return v;
}
unsigned short cleanup(unsigned char x) {
 unsigned char i;
 unsigned short v=0;
 if(x&1) goto fail;
 mark(1);
 for(i=0;i<4;i++) { if((x&7)==i) goto release; mark(i+2); v+=i; }
release:
 mark(9); return v+100;
fail:
 mark(15); return 255;
}
unsigned short multi_entry(unsigned char x) {
 unsigned char i=0, limit=(x&3)+2;
 unsigned short v=0;
 if(x&1) goto right;
left:
 mark(16); v+=i+3; i++;
 if(i>=limit) goto done;
 if(x&(1u<<i)) goto left;
 goto right;
right:
 mark(32); v+=2*i+5; i++;
 if(i>=limit) goto done;
 if(x&(1u<<i)) goto left;
 goto right;
done:
 mark(48); return v;
}
unsigned short scope_exit(unsigned char x) {
 unsigned char i=0, limit=(x&3)+1;
 unsigned short v=0;
 for(;i<limit;i++) {
  { unsigned char i=7; mark(i); v+=i; if(x&4) goto done; }
  mark(i);
 }
done:
 mark(48); return v*16+i;
}
unsigned short skip_nested(unsigned char x) {
 unsigned char i,j;
 unsigned short v=0;
 for(i=0;i<3;i++) {
  j=0;
  while(j<3) { j++; if((x+i+j)%5==0) goto next; mark(i*4+j); v+=i+j; }
next:
  v+=10;
 }
 return v;
}
void execute_loops(void) { result=loops(input); }
void execute_cleanup(void) { result=cleanup(input); }
void execute_multi_entry(void) { result=multi_entry(input); }
void execute_scope_exit(void) { result=scope_exit(input); }
void execute_skip_nested(void) { result=skip_nested(input); }
