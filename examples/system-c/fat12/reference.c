/* Independent oracle: upstream FatFS R0.16, compiled by the host C compiler.
 * This is a runner for ff.c, not the Z80 example's small FAT12 reader. */
#include <stdio.h>
#include "ff.h"
#include "diskio.h"
static FILE *image;
DSTATUS disk_initialize(BYTE drive) {return drive ? STA_NOINIT : 0;}
DSTATUS disk_status(BYTE drive) {return drive ? STA_NOINIT : 0;}
DRESULT disk_read(BYTE drive, BYTE *buffer, LBA_t sector, UINT count) {
 if (drive || sector+count>2880) return RES_PARERR;
 if (fseek(image,(long)sector*512,SEEK_SET)!=0) return RES_ERROR;
 return fread(buffer,512,count,image)==count ? RES_OK : RES_ERROR;
}
DRESULT disk_write(BYTE drive, const BYTE *buffer, LBA_t sector, UINT count) {return RES_WRPRT;}
DRESULT disk_ioctl(BYTE drive, BYTE cmd, void *buffer) {
 if (drive) return RES_PARERR;
 switch (cmd) {
 case CTRL_SYNC:return RES_OK;
 case GET_SECTOR_COUNT:*(LBA_t *)buffer=2880;return RES_OK;
 case GET_SECTOR_SIZE:*(WORD *)buffer=512;return RES_OK;
 case GET_BLOCK_SIZE:*(DWORD *)buffer=1;return RES_OK;
 default:return RES_PARERR;
 }
}
DWORD get_fattime(void) {return 0;}
int main(int argc,char **argv) {
 FATFS fs; FIL file; BYTE data[1024]; UINT count; FRESULT err;
 if (argc!=2 || !(image=fopen(argv[1],"rb"))) return 2;
 err=f_mount(&fs,"",1);if(err) {fprintf(stderr,"mount %d\n",err);return 3;}
 err=f_open(&file,"HELLO.TXT",FA_READ);if(err) {fprintf(stderr,"open %d\n",err);return 4;}
 do {err=f_read(&file,data,sizeof data,&count);if(err)return 5;if(fwrite(data,1,count,stdout)!=count)return 6;}while(count);
 f_close(&file);fclose(image);return 0;
}
