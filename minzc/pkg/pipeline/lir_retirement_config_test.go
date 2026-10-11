package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
)

// Previous saved configurations contained the two false LIR fields. Their
// model must be rejected rather than silently reassigned to today's compiler.
func TestRetiredLIRConfigurationRefused(t *testing.T) {
	options := DefaultOptions()
	encoded, err := json.Marshal(options)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("LIRCheck")) || bytes.Contains(encoded, []byte("UseLIR")) {
		t.Fatal("retired fields remain in current configuration")
	}
	legacy := bytes.Replace(encoded, []byte(`"AnnotateTStates":false`), []byte(`"AnnotateTStates":false,"LIRCheck":false,"UseLIR":false`), 1)
	legacyEnvelope := append([]byte(`{"Options":`), legacy...)
	legacyEnvelope = append(legacyEnvelope, []byte(`,"AssemblerMode":"default"}`)...)
	oldID := fmt.Sprintf("%x", sha256.Sum256(legacyEnvelope))
	currentID, err := ProfileCompilationID(options, "default")
	if err != nil || currentID == oldID {
		t.Fatalf("configuration identity unchanged: %s %s %v", currentID, oldID, err)
	}
	options.ProfileSelection = &ProfileSelection{AssemblerMode: "default", Model: hir.LoopCostModel{CompilationID: oldID}}
	if _, err := applyPipelineProfile(&hir.Module{}, options); err == nil || !strings.Contains(err.Error(), "measured compilation configuration mismatch") {
		t.Fatalf("old model not refused: %v", err)
	}
	t.Logf("legacy_config_id=%s current_config_id=%s", oldID, currentID)
}
