package z80asm

import (
	"bytes"
	"strings"
	"testing"
)

func TestAgonZ80PackageContract(t *testing.T) {
	for _, prefix := range []string{"", "ORG $45\n"} {
		a := NewAssembler()
		if err := a.SetTarget(Target("agon-z80")); err != nil {
			t.Fatal(err)
		}
		r, err := a.AssembleString(prefix + "start:\nCALL body\nLD HL,payload\nRET.LIS\nbody:\nRET\npayload:\nDB $A5\n")
		if err != nil || len(r.Errors) != 0 {
			t.Fatal(err, r)
		}
		image, err := GetTargetConfig(Target("agon-z80")).OutputFormat.Generator(r)
		if err != nil {
			t.Fatal(err)
		}
		if r.Origin != 69 || !bytes.Equal(image[:3], []byte{0xC3, 69, 0}) || !bytes.Equal(image[64:69], []byte{'M', 'O', 'S', 0, 0}) || !bytes.Equal(image[69:], r.Binary) {
			t.Fatalf("invalid mode0 package origin=%x image=%x", r.Origin, image)
		}
		if int(image[70])|int(image[71])<<8 != r.Symbols["body"] || int(image[73])|int(image[74])<<8 != r.Symbols["payload"] || image[r.Symbols["payload"]] != 0xA5 {
			t.Fatal("logical CALL/data addresses disagree with physical bank offsets")
		}
	}
}

func TestAgonZ80PackageBounds(t *testing.T) {
	for _, src := range []string{
		"ORG $40045\nRET\n", "ORG 0\nRET\n", "ORG $46\nRET\n",
		"ORG $45\nRET\nORG $60\nDB 0\n", "ORG $45\nRET\nEND 0\n",
		"ORG $45\nDS $F000-$+1,0\n", // keep upper bank page/caller words out of load image
	} {
		a := NewAssembler()
		if err := a.SetTarget(Target("agon-z80")); err != nil {
			t.Fatal(err)
		}
		r, err := a.AssembleString(src)
		if err != nil || len(r.Errors) != 0 {
			if !strings.Contains(src, "$40045") && !strings.Contains(src, "$F000") {
				t.Fatal(err, r)
			}
			continue
		}
		if _, err := GetTargetConfig(Target("agon-z80")).OutputFormat.Generator(r); err == nil {
			t.Fatalf("packaged incompatible mode0 image: %s", src)
		}
	}
}

func TestAgonZ80ExactImageCeilingAndExplicitEntry(t *testing.T) {
	a := NewAssembler()
	if e := a.SetTarget(TargetAgonZ80); e != nil {
		t.Fatal(e)
	}
	r, e := a.AssembleString("DB 0\nstart: RET.LIS\nDS $F000-$,0\nEND start\n")
	if e != nil || len(r.Errors) > 0 {
		t.Fatal(e, r.Errors)
	}
	image, e := generateAgonZ80Bin(r)
	if e != nil || len(image) != AgonZ80ImageLimit || !bytes.Equal(image[:3], []byte{0xC3, 70, 0}) {
		t.Fatalf("exact bound/explicit entry refused: %v length%d", e, len(image))
	}
}
