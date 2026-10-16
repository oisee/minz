unsigned short fallthrough(unsigned char x) {
    unsigned short v = 0;
    switch (x & 3) {
    case 0: v += 1;
    case 1: v += 2; break;
    case 2: v += 4;
    default: v += 8;
    }
    return v;
}
unsigned short loop_in_switch(unsigned char x) {
    unsigned short v = 0;
    unsigned char i;
    switch (x & 1) {
    case 0:
        for(i=0;i<4;i++) { v += 1; if(i==1) break; }
        v += 10;
        break;
    default: v=20;
    }
    return v;
}
unsigned short goto_scan(unsigned char x) {
    unsigned short v = 0;
    unsigned char i = 0;
again:
    if(i>=x) goto done;
    v += i;
    i++;
    goto again;
done:
    return v;
}
unsigned short default_middle(unsigned char x) {
    unsigned short v=0;
    switch(x) {
    case 0: v+=1;
    default: v+=2;
    case 1: v+=4; break;
    case 2: v+=8;
    }
    return v;
}
unsigned short chained_labels(unsigned char x) {
    switch(x) { case 0: case 1: return 3; default: return 7; }
}
unsigned short nested_switch(unsigned char x) {
    unsigned short v=0;
    switch(x&3) {
    case 0:
        switch(x&4) {case 0:v=1;break;default:v=2;}
        v+=10;
        break;
    default:v=30;
    }
    return v;
}
unsigned short switch_continue(unsigned char x) {
    unsigned short v=0;
    unsigned char i;
    for(i=0;i<6;i++) {
        switch((i+x)&3) {
        case 1: continue;
        case 2: v+=2;break;
        default:v+=3;
        }
        v+=10;
    }
    return v;
}
unsigned char selector;
unsigned short selector_once(unsigned char x) {
    unsigned short v=0;
    selector=x;
    switch(selector++) {case 0:v=1;break;case 1:v=2;break;default:v=3;}
    return (unsigned short)(v+(unsigned short)selector*16u);
}
unsigned short byte_case(unsigned char x) {
    switch(x) {case 255:return 1;case -1:return 2;default:return 3;}
}
unsigned short join_unused(unsigned char x) {
    unsigned short a=0, b=0;
    if(x&1) a=1;
    else { switch(x) {case 0:a=2;b=3;break;default:a=4;} }
    return a;
}
