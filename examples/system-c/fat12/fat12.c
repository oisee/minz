#import "block_device.nanz"
#include "fat12.h"

BYTE fat_sector[512];
BYTE fat_seen[512];
BYTE fat_error;
WORD fat_start, fat_root, fat_data, fat_root_entries, fat_clusters;
BYTE fat_spc;
WORD fat_cluster, fat_cluster_sector;
DWORD fat_size, fat_remaining;

static WORD word_at(WORD n) {
 return (WORD)fat_sector[n] | ((WORD)fat_sector[n+1] << 8);
}
static DWORD dword_at(WORD n) {
 return (DWORD)word_at(n) | ((DWORD)word_at(n+2) << 16);
}
static BYTE sector_read(WORD n) {
 if (block_read(n,fat_sector)!=0) {fat_error=1;return 1;}
 return 0;
}
BYTE fat_mount(void) {
 WORD total,fat_sectors,root_sectors;
 DWORD root,data;
 fat_error=0;
 if (sector_read(0)) return fat_error;
 if (word_at(11)!=512 || word_at(510)!=0xaa55) {fat_error=2;return 2;}
 fat_spc=fat_sector[13];
 if (fat_spc==0 || fat_spc>8 || (fat_spc&(fat_spc-1))!=0) {fat_error=2;return 2;}
 fat_start=word_at(14);
 fat_root_entries=word_at(17);
 total=word_at(19);
 fat_sectors=word_at(22);
 if (fat_start==0 || (fat_sector[16]!=1 && fat_sector[16]!=2) || fat_sectors==0 || total==0 || fat_root_entries==0) {fat_error=2;return 2;}
 root_sectors=(WORD)(((DWORD)fat_root_entries+15UL)>>4);
 root=(DWORD)fat_start+fat_sectors;
 if (fat_sector[16]==2) root=root+fat_sectors;
 data=root+root_sectors;
 if (data>=total) {fat_error=2;return 2;}
 fat_root=(WORD)root;fat_data=(WORD)data;
 fat_clusters=(total-fat_data)/fat_spc;
 if (fat_clusters==0 || fat_clusters>=4085 || fat_sectors<((fat_clusters+2)*3+1023)/1024) {fat_error=2;return 2;}
 return 0;
}
BYTE fat_find(const BYTE *name11) {
 WORD entry,off,i;
 BYTE match;
 fat_error=0;
 for (entry=0;entry<fat_root_entries;entry++) {
  off=(entry&15)*32;
  if (off==0 && sector_read(fat_root+entry/16)) return fat_error;
  if (fat_sector[off]==0) {fat_error=3;return 3;}
  if (fat_sector[off]==0xe5 || (fat_sector[off+11]&0x18)!=0) continue;
  match=1;
  for (i=0;i<11;i++) if (fat_sector[off+i]!=name11[i]) match=0;
  if (match) {
   fat_cluster=word_at(off+26);
   fat_size=dword_at(off+28);
   fat_remaining=fat_size;
   fat_cluster_sector=0;
   if (fat_size!=0 && (fat_cluster<2 || fat_cluster>=fat_clusters+2)) {fat_error=4;return 4;}
   for (i=0;i<512;i++) fat_seen[i]=0;
   if (fat_size!=0) fat_seen[fat_cluster>>3]=(BYTE)(1<<(fat_cluster&7));
   return 0;
  }
 }
 fat_error=3;return 3;
}
static WORD next_cluster(WORD cluster) {
 WORD off,lo,hi,value;
 off=cluster+cluster/2;
 if (sector_read(fat_start+off/512)) return 0;
 lo=fat_sector[off&511];
 if ((off&511)==511) {
  if (sector_read(fat_start+off/512+1)) return 0;
  hi=fat_sector[0];
 } else hi=fat_sector[(off&511)+1];
 value=lo|(hi<<8);
 if (cluster&1) value=value>>4;
 return value&0xfff;
}
WORD fat_read(BYTE *dst) {
 WORD i,n,next;
 if (fat_error || fat_remaining==0) return 0;
 if (fat_cluster<2 || fat_cluster>=fat_clusters+2) {fat_error=4;return 0;}
 if (sector_read(fat_data+(fat_cluster-2)*fat_spc+fat_cluster_sector)) return 0;
 n=512;
 if (fat_remaining<512UL) n=(WORD)fat_remaining;
 for (i=0;i<n;i++) dst[i]=fat_sector[i];
 fat_remaining=fat_remaining-n;
 fat_cluster_sector++;
 if (fat_remaining!=0 && fat_cluster_sector==fat_spc) {
  fat_cluster_sector=0;
  next=next_cluster(fat_cluster);
  if (fat_error) return 0;
  if (next<2 || next>=0xff0 || next>=fat_clusters+2) {fat_error=4;return 0;}
  if (fat_seen[next>>3] & (1<<(next&7))) {fat_error=4;return 0;}
  fat_seen[next>>3]|=(BYTE)(1<<(next&7));
  fat_cluster=next;
 }
 return n;
}
