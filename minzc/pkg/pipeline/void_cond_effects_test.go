package pipeline_test

import (
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"testing"
)

func TestVoidMatchEffectsVMZ80(t *testing.T) {
	src := `global cell:u8=0
 enum Choice { Left, Right }
 fun left() { cell=17 }
 fun right() { cell=91 }
 fun choose(c:Choice) -> u8 {
  match c { Left => left(), Right => right(), _ => right() }
  return cell
 }
 fun first() -> u8 { return choose(Choice.Left) }
 fun second() -> u8 { return choose(Choice.Right) }
 assert first()==17
 assert second()==91`
	h, err := nanz.Parse(src, "void_match")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pipeline.CompileHIR(h); err != nil {
		t.Fatal(err)
	}
}
