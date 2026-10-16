/* Unmodified decoder, upstream-supported static configuration. */
#define HEATSHRINK_DYNAMIC_ALLOC 0
#include "../upstream/heatshrink/heatshrink_decoder.c"
unsigned char input[32];
unsigned char output[256];
unsigned int input_length;
unsigned int result;
unsigned int trace[5];
static heatshrink_decoder decoder;
void execute(void) {
    size_t sunk, produced;
    unsigned int i;
    HSD_sink_res sink;
    HSD_poll_res poll;
    HSD_finish_res finish;
    for (i=0;i<256;i++) output[i]=0;
    heatshrink_decoder_reset(&decoder);
    sink=heatshrink_decoder_sink(&decoder,input,input_length,&sunk);
    poll=heatshrink_decoder_poll(&decoder,output,256,&produced);
    finish=heatshrink_decoder_finish(&decoder);
    trace[0]=sink; trace[1]=sunk; trace[2]=poll; trace[3]=produced; trace[4]=finish;
    if (sink!=HSDR_SINK_OK || sunk!=input_length || poll<0 || finish!=HSDR_FINISH_DONE) result=65535;
    else result=produced;
}

#include "target_string.c"
