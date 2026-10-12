#define _GNU_SOURCE
#include <dlfcn.h>
#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <stdbool.h>
static void *original;
static void *sym(const char *name){if(!original){original=dlopen(getenv("MINZ_REAL_VDP"),RTLD_NOW|RTLD_LOCAL);if(!original){fprintf(stderr,"VDP tee dlopen: %s\n",dlerror());abort();}}void *p=dlsym(original,name);if(!p){fprintf(stderr,"VDP tee symbol %s: %s\n",name,dlerror());abort();}return p;}
#define VOID0(name) void name(void){((void(*)(void))sym(#name))();}
VOID0(vdp_setup) VOID0(vdp_loop) VOID0(vdp_shutdown) VOID0(signal_vblank) VOID0(dump_vdp_mem_stats)
void z80_send_to_vdp(uint8_t b){static FILE *f;if(!f){f=fopen(getenv("MINZ_VDP_STREAM"),"wb");if(!f)abort();}fputc(b,f);fflush(f);((void(*)(uint8_t))sym("z80_send_to_vdp"))(b);}
bool z80_uart0_is_cts(void){return ((bool(*)(void))sym("z80_uart0_is_cts"))();}
bool z80_recv_from_vdp(uint8_t *b){return ((bool(*)(uint8_t*))sym("z80_recv_from_vdp"))(b);}
void set_startup_screen_mode(uint32_t m){((void(*)(uint32_t))sym("set_startup_screen_mode"))(m);}
void setVdpDebugLogging(bool b){((void(*)(bool))sym("setVdpDebugLogging"))(b);}
void copyVgaFramebuffer(uint32_t *w,uint32_t *h,uint8_t *buf,float *fps){((void(*)(uint32_t*,uint32_t*,uint8_t*,float*))sym("copyVgaFramebuffer"))(w,h,buf,fps);}
void sendVKeyEventToFabgl(uint32_t key,uint8_t down){((void(*)(uint32_t,uint8_t))sym("sendVKeyEventToFabgl"))(key,down);}
void sendPS2KbEventToFabgl(uint16_t key,uint8_t down){((void(*)(uint16_t,uint8_t))sym("sendPS2KbEventToFabgl"))(key,down);}
void sendHostMouseEventToFabgl(const uint8_t *packet){((void(*)(const uint8_t*))sym("sendHostMouseEventToFabgl"))(packet);}
void getAudioSamples(uint8_t *buf,uint32_t length){((void(*)(uint8_t*,uint32_t))sym("getAudioSamples"))(buf,length);}
