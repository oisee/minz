#define _GNU_SOURCE
#include <SDL3/SDL.h>
#include <dlfcn.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>
static unsigned count=0;
bool SDL_UpdateTexture(SDL_Texture *t,const SDL_Rect *r,const void *pixels,int pitch){
 bool (*real)(SDL_Texture*,const SDL_Rect*,const void*,int)=dlsym(RTLD_NEXT,"SDL_UpdateTexture");
 float w=0,h=0;SDL_GetTextureSize(t,&w,&h);++count;
 if(count==240 && w>0 && h>0 && pitch==(int)w*3){
  const char *path=getenv("MINZ_AGON_FRAME");if(path){FILE *f=fopen(path,"wb");if(f){fprintf(f,"P6\n%d %d\n255\n",(int)w,(int)h);fwrite(pixels,1,pitch*(int)h,f);fclose(f);}}
 }
 return real(t,r,pixels,pitch);
}
