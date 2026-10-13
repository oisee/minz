"""Disk MZ3R transport through emulator TR-DOS API; RAM slot is temporary staging.

Dedicated TRD tracks2..5, single slot; payload sectors first, header last.
Existing preflight validates identity/CRC/structure before applying game state.
"""
def emit(bank,offset,capacity):
    return f'''import zx.scorpion_memory
import zx.scorpion_disk
global disk_record_length:u16
fun native_disk_copy_out(base:u16,count:u16) {{
 var i:u16=0
 while i<256 {{
  var value:u8=0
  if base+i<count {{ value=bank_read({bank},{offset}+base+i,0x10) }}
  scorpion_write(8,i,value,0x10,0)
  i=i+1
 }}
}}
fun native_disk_save()->u8 {{
 if native_save()==0 {{ return 0 }}
 disk_record_length=native_save_length
 let sectors:u16=(disk_record_length+255)>>8
 var sector:u16=1
 while sector<sectors {{
  native_disk_copy_out(sector<<8,disk_record_length)
  if scorpion_disk_sector(6,0,u8(2+(sector>>4)),u8((sector&15)+1))==0 {{ return 0 }}
  sector=sector+1
 }}
 native_disk_copy_out(0,disk_record_length)
 return scorpion_disk_sector(6,0,2,1)
}}
fun native_disk_restore()->u8 {{
 if scorpion_disk_sector(5,0,2,1)==0 {{ return 0 }}
 // MZ3R stores two little-endian encoded lengths at offsets12 and14.
 let first:u16=u16(scorpion_read(8,12,0x10,0)) | (u16(scorpion_read(8,13,0x10,0))<<8)
 let second:u16=u16(scorpion_read(8,14,0x10,0)) | (u16(scorpion_read(8,15,0x10,0))<<8)
 if first>{capacity}-32 {{ return 0 }}
 if second>{capacity}-32-first {{ return 0 }}
 disk_record_length=first+second+32
 var base:u16=0
 while base<disk_record_length {{
  if base!=0 {{
   let sector:u16=base>>8
   if scorpion_disk_sector(5,0,u8(2+(sector>>4)),u8((sector&15)+1))==0 {{ return 0 }}
  }}
  var i:u16=0
  while i<256 {{
   if base+i<disk_record_length {{ bank_write({bank},{offset}+base+i,scorpion_read(8,i,0x10,0),0x10) }}
   i=i+1
  }}
  base=base+256
 }}
 return native_restore({bank},{offset},disk_record_length)
}}
'''
