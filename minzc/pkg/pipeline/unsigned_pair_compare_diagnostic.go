package pipeline

import "github.com/minz/minzc/pkg/mir2"

// Private serial-test controls on this isolated branch; never public Options.
var unsignedPairExperimentEnabled bool
var unsignedPairExperimentMIRHook func(*mir2.Module) error
