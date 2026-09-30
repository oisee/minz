package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/oisee/z80-optimizer/pkg/ayumi"
	"math"
	"os"
)

func render(envelopeFirst bool) ([]float64, string) {
	r := ayumi.NewRenderAyumi(ayumi.ChipAY, 1773400, 44100)
	r.SetPan(0, 0.5)
	r.WriteRegister(0, 100)
	r.WriteRegister(1, 0)
	r.WriteRegister(11, 10)
	r.WriteRegister(12, 0)
	r.WriteRegister(13, 10)
	if envelopeFirst {
		r.WriteRegister(8, 16)
		r.WriteRegister(7, 62)
	} else {
		r.WriteRegister(7, 62)
		r.WriteRegister(8, 16)
	}
	samples := make([]float64, 4410)
	h := sha256.New()
	for i := range samples {
		l, _ := r.GenerateSample()
		samples[i] = l
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], math.Float64bits(l))
		h.Write(b[:])
	}
	return samples, fmt.Sprintf("%x", h.Sum(nil))
}
func main() {
	a, ha := render(false)
	b, hb := render(true)
	_, hc := render(false)
	sum, peak := 0., 0.
	nonzero := 0
	diff := 0
	for i, x := range a {
		sum += x * x
		peak = math.Max(peak, math.Abs(x))
		if x != 0 {
			nonzero++
		}
		if x != b[i] {
			diff++
		}
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"samples": len(a), "rms": math.Sqrt(sum / float64(len(a))), "peak": peak, "nonzero": nonzero, "repeat_identical": ha == hc, "mixer_then_envelope_hash": ha, "envelope_then_mixer_hash": hb, "order_different_samples": diff})
}
