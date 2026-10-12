package main

import (
	"github.com/spf13/pflag"
	"testing"
)

func TestRetiredLIRFlagsRejected(t *testing.T) {
	for _, name := range []string{"lir", "z3"} {
		flags := pflag.NewFlagSet("retired", pflag.ContinueOnError)
		flags.AddFlagSet(rootCmd.Flags())
		if err := flags.Parse([]string{"--" + name}); err == nil {
			t.Fatalf("retired flag --%s accepted", name)
		}
	}
}
