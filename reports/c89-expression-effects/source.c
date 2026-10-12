unsigned char trace;
unsigned char mark(unsigned char v) { trace=trace*3+v; return v; }
unsigned int and_post(unsigned char yes,unsigned char start) { unsigned char n=start,x; x=yes && n++; return (unsigned int)n*256+x; }
unsigned int or_post(unsigned char yes,unsigned char start) { unsigned char n=start,x; x=yes || n++; return (unsigned int)n*256+x; }
unsigned int pick_post(unsigned char yes,unsigned char start) { unsigned char n=start,x; x=yes ? n++ : ++n; return (unsigned int)n*256+x; }
unsigned int effect_cond(unsigned char yes,unsigned char start) { unsigned char n=start,x; x=yes++ ? n++ : ++n; return (unsigned int)yes*256+x; }
unsigned int nested(unsigned char yes,unsigned char start) { unsigned char n=start,x; x=(yes && n++) ? ++n : n++; return (unsigned int)n*256+x; }
unsigned char trace_and(unsigned char yes) { trace=0; return mark(yes) && mark(7); }
unsigned char trace_or(unsigned char yes) { trace=0; return mark(yes) || mark(7); }
unsigned char trace_pick(unsigned char yes) { trace=0; return mark(yes) ? mark(7) : mark(9); }
unsigned char loop_condition(unsigned char limit) { unsigned char n=0,sum=0; while(n<limit && n++<3) sum+=n; return sum; }
