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
 if((count==330 || count==420) && w>0 && h>0 && pitch==(int)w*3){
  const char *path=getenv(count==330 ? "MINZ_AGON_FRAME" : "MINZ_AGON_RETURN_FRAME");if(path){FILE *f=fopen(path,"wb");if(f){fprintf(f,"P6\n%d %d\n255\n",(int)w,(int)h);fwrite(pixels,1,pitch*(int)h,f);fclose(f);}}
 }
 unsigned sc=0;SDL_Keycode key=0;bool down=false;
 if(count==270||count==272){sc=SDL_SCANCODE_DOWN;key=SDLK_DOWN;down=count==270;}
 if(count==300||count==302){sc=SDL_SCANCODE_RETURN;key=SDLK_RETURN;down=count==300;}
 if(count==360||count==362){sc=SDL_SCANCODE_ESCAPE;key=SDLK_ESCAPE;down=count==360;}
 if(sc){SDL_Event e={0};e.type=down?SDL_EVENT_KEY_DOWN:SDL_EVENT_KEY_UP;e.key.scancode=sc;e.key.key=key;e.key.down=down;SDL_PushEvent(&e);}
 return real(t,r,pixels,pitch);
}
