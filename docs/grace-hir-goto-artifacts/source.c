unsigned short skip(void){int x=1;goto done;x=99;done:return x;}
unsigned short outward(void){int x=7;int i=0;again:{i=i+1;int x=99;if(i<2)goto again;}return x;}
unsigned short forward(void){int x=7;{x=9;int x=99;goto done;}x=88;done:return x;}
unsigned short static_shadow(void){static int x=7;{int x=99;x=x+1;}return x;}
unsigned short dead_both(void){int x=7;goto b;if(0){a:x=9;goto out;}else{b:x=11;goto out;}out:return x;}
unsigned short loop_entry(void){goto a;for(int i=0;i<3;i++){a:return 7;}return 0;}
unsigned short dead_switch(void){int x=7;goto a;switch(x){case 7:a:x=9;break;default:x=3;}return x;}
unsigned short branch_assign(void){goto a;int x=99;a:if(1)x=7;else x=8;return x;}
unsigned short return_assign(void){goto a;int x=99;a:if(0){return 7;}else{x=2;}return x;}
unsigned short assigned_before_shadow(void){goto a;int x=99;a:if(1){x=2;int x=3;x=4;}else{x=9;}return x;}
// assert static_shadow() == 7
// assert skip() == 1
// assert outward() == 7
// assert forward() == 9
// assert dead_both() == 11
// assert loop_entry() == 7
// assert dead_switch() == 9
// assert branch_assign() == 7
// assert return_assign() == 2
// assert assigned_before_shadow() == 2
