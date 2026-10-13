"""Disk MZ3R transport through emulator TR-DOS API; RAM slot is temporary staging.

Dedicated TRD marker track1/sector1 and two slots tracks2..5 and6..9.
Commit marker published last; atomic sector persistence is provided by emulator.
Existing preflight validates identity/CRC/structure before applying game state.
"""
def emit(bank,offset,capacity):
    return f'''import zx.scorpion_memory
import zx.scorpion_disk
global disk_record_length:u16
global disk_slot:u8
fun native_disk_marker()->u8 {{
 if scorpion_disk_sector(5,0,1,1)==0 {{ return 0 }}
 if scorpion_read(8,0,0x10,0)!=77 {{ return 0 }}
 if scorpion_read(8,1,0x10,0)!=90 {{ return 0 }}
 disk_slot=scorpion_read(8,2,0x10,0)
 if disk_slot>1 {{ return 0 }}
 if scorpion_read(8,3,0x10,0)!=(disk_slot xor 255) {{ return 0 }}
 return 1
}}
fun native_disk_commit()->u8 {{
 var i:u16=0
 while i<256 {{ scorpion_write(8,i,0,0x10,0) i=i+1 }}
 scorpion_write(8,0,77,0x10,0)
 scorpion_write(8,1,90,0x10,0)
 scorpion_write(8,2,disk_slot,0x10,0)
 scorpion_write(8,3,disk_slot xor 255,0x10,0)
 return scorpion_disk_sector(6,0,1,1)
}}
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
 if native_disk_marker()!=0 {{ disk_slot=disk_slot xor 1 }}
 else {{ disk_slot=0 }}
 disk_record_length=native_save_length
 let sectors:u16=(disk_record_length+255)>>8
 var sector:u16=1
 while sector<sectors {{
  native_disk_copy_out(sector<<8,disk_record_length)
  if scorpion_disk_sector(6,0,u8(2+u16(disk_slot)*4+(sector>>4)),u8((sector&15)+1))==0 {{ return 0 }}
  sector=sector+1
 }}
 native_disk_copy_out(0,disk_record_length)
 if scorpion_disk_sector(6,0,2+disk_slot*4,1)==0 {{ return 0 }}
 return native_disk_commit()
}}
fun native_disk_restore()->u8 {{
 if native_disk_marker()==0 {{ return 0 }}
 if scorpion_disk_sector(5,0,2+disk_slot*4,1)==0 {{ return 0 }}
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
   if scorpion_disk_sector(5,0,u8(2+u16(disk_slot)*4+(sector>>4)),u8((sector&15)+1))==0 {{ return 0 }}
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
