import unittest
from restart_lz import pack,unpack

class LZTests(unittest.TestCase):
 def test_roundtrip(self):
  for size in [0,1,2,3,7,8,9,17,18,19,127,128,4096,4097,8192,16384]:
   for data in [bytes(size),bytes((i*37+(i//31))&255 for i in range(size)),bytes(i%251 for i in range(size))]:
    self.assertEqual(unpack(pack(data),size),data)
 def test_overlap(self):
  self.assertEqual(unpack(bytes([1,65,15,0]),19),b'A'*19)
 def test_invalid(self):
  for data,size in [(b'',1),(b'\x01',1),(b'\x00\x00',3),(b'\x00\x00\x00',3),(b'\x01A\x0f\x00',18),(b'\x01A\x10\x00',4),(b'x',0)]:
   with self.assertRaises(ValueError):unpack(data,size)
 def test_truncation(self):
  data=b'abcabcabcabc'*17;encoded=pack(data)
  for end in range(len(encoded)):
   with self.assertRaises(ValueError):unpack(encoded[:end],len(data))

if __name__=='__main__':unittest.main()
