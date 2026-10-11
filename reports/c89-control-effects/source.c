unsigned char state;
unsigned char while_post(unsigned char limit) { unsigned char n=0, sum=0; while(n++<limit) sum+=n; state=n; return sum; }
unsigned char do_post(unsigned char limit) { unsigned char n=0, sum=0; do { sum+=n; } while(n++<limit); state=n; return sum; }
unsigned char for_post(unsigned char limit) { unsigned char n=0, sum=0; for(;n++<limit;) sum+=n; state=n; return sum; }
unsigned char choose(unsigned char yes) { unsigned char n=7; if(yes) return n++; state=n; return n; }
unsigned char choose_else(unsigned char yes) { unsigned char n=7; if(yes) return n++; else return ++n; }
unsigned char for_continue(unsigned char limit) { unsigned char n=0,sum=0; for(;n<limit;n++) { if(n==2) continue; sum+=n; } state=n; return sum; }
unsigned char do_continue(unsigned char limit) { unsigned char n=0,sum=0; do { if(n==2) continue; sum+=n; } while(n++<limit); state=n; return sum; }
unsigned char while_call(unsigned char limit) { unsigned char n=0,sum=0; while(++n<limit) { if(n==2) continue; sum+=n; } state=n; return sum; }
