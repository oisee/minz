import unittest
from story_lz_layout import build,reconstruct,BANK_SIZE

class LayoutTests(unittest.TestCase):
 def story(self,size,dynamic):
  data=bytearray((i*37+i//31)&255 for i in range(size))
  data[0]=3;data[14:16]=dynamic.to_bytes(2,'big')
  return bytes(data)
 def test_bounds_and_reserved_save(self):
  for size,dynamic in [(64,64),(32700,1024),(86928,8192),(98304,16384)]:
   story=self.story(size,dynamic)
   for block in [256,2048,4096]:
    image,m=build(story,block,2048,5)
    self.assertEqual(reconstruct(image,m),story)
    self.assertEqual(len(image),98304)
    occupied={(0,i) for i in range(dynamic)}
    for obj in m['objects']:
     self.assertLessEqual(obj['offset']+obj['stored_bytes'],BANK_SIZE)
     if obj['name']=='restart':self.assertNotEqual(obj['page'],0)
     for i in range(obj['offset'],obj['offset']+obj['stored_bytes']):
      self.assertNotIn((obj['page'],i),occupied)
      occupied.add((obj['page'],i))
    for i in range(14336,16384):self.assertNotIn((5,i),occupied)
    self.assertEqual(build(story,block,2048,5),(image,m))
    for strategy in range(1,7):
     alt,meta=build(story,block,2048,5,strategy)
     self.assertEqual(reconstruct(alt,meta),story)
 def test_original_blocks(self):
  story=self.story(70000,8192)
  image,m=build(story,2048,512,5,3,True)
  self.assertEqual(reconstruct(image,m),story)
  self.assertTrue(m['archive_blocks'])
  self.assertEqual(m['original_index_bytes'],16)
  self.assertFalse(any(o['name']=='restart' for o in m['objects']))
  self.assertTrue(all(o['page']!=0 for o in m['objects'] if o['name'].startswith('original')))
 def test_impossible_reservation(self):
  import random
  data=bytearray(random.Random(42).randbytes(98304))
  data[0]=3;data[14:16]=(16384).to_bytes(2,'big')
  with self.assertRaises(ValueError):build(bytes(data),2048,16384,5)
 def test_invalid(self):
  for data in [b'',b'\x03'*63,self.story(65536,32768)]:
   with self.assertRaises(ValueError):build(data,2048)
  for reserve,page in [(-1,5),(16385,5),(0,0),(0,6)]:
   with self.assertRaises(ValueError):build(self.story(64,64),2048,reserve,page)

if __name__=='__main__':unittest.main()
