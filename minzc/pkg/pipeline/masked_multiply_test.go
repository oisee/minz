package pipeline

import "testing"

func TestMaskedByteMultiplyRepro(t *testing.T) {
	const src = `fun mul8x8(x:u8,y:u8)->u16 {
 var multiplicand:u16=u16(x)
 var multiplier:u8=y
 var product:u16=0
 while multiplier!=0 {
  if (multiplier&1)!=0 {product=product+multiplicand}
  multiplicand=multiplicand<<1
  multiplier=multiplier>>1
 }
 return product
 }`
	for _, v := range [][3]int64{{1, 2, 2}, {3, 5, 15}, {255, 255, 65025}, {7, 6, 42}} {
		measureGraceZ80(t, src, "mul8x8", false, []int64{v[0], v[1]}, nil, v[2])
	}
}
