package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/nanz"
)

func TestInvariantAddressReassociationNativeWrap(t *testing.T) {
	const source = `fun copy(base:^u8,out:^u8,ofs:u16,n:u8)->u8{
 var i:u8=0
 var acc:u8=0
 while i<n {
 out[u16(i)]=base[ofs+u16(i)]
 acc=acc*3+out[u16(i)]
 i=i+1
 }
 return acc
 }`
	for _, size := range []bool{false, true} {
		for _, n := range []int64{0, 1, 3} {
			h, err := nanz.Parse(source, "address_wrap.nanz")
			if err != nil {
				t.Fatal(err)
			}
			want := int64(0)
			for i := int64(0); i < n; i++ {
				want = want*3 + int64([]byte{7, 8, 9}[i])
			}
			measureGraceZ80HIR(t, h, "copy", Options{ContractOpt: true, UseGrace: true, OptSize: size, AssertMode: "none"}, []int64{65524, 0x4100, 11, n}, map[uint16]byte{65535: 7, 0: 8, 1: 9}, want)
		}
	}
}
