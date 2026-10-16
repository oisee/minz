package c89_test

import (
	"github.com/minz/minzc/pkg/c89"
	"strings"
	"testing"
)

func TestCRejectsUnsupportedBitfieldStorageAndAccess(t *testing.T) {
	for _, source := range []string{
		"struct B{unsigned int value:3;};struct B bits;unsigned int f(void){bits.value=9;return bits.value;}",
		"struct B{unsigned int value:3;};unsigned int f(struct B *bits){return bits->value;}",
		"struct B{unsigned int value:3;};unsigned int f(struct B *bits){bits->value+=1;return 0;}",
		"struct B{unsigned int value:3;};unsigned int f(struct B *bits){return bits->value++;}",
		"struct B{unsigned int value:3;};unsigned int f(struct B *bits){return ++bits->value;}",
		"struct B{unsigned int value:3;};unsigned int f(void){struct B bits={1};return bits.value;}",
		"union B{unsigned int value:3;unsigned char bytes[2];};union B bits;unsigned int f(void){return bits.value;}",
		"typedef struct {unsigned int value:3;} B;B bits;unsigned int f(void){return 0;}",
	} {
		if _, err := c89.Compile(source, "unsupported-bits.c"); err == nil || !strings.Contains(err.Error(), "bit-field") {
			t.Fatalf("expected explicit bit-field diagnostic: %v", err)
		}
	}
}
