package pipeline

import "github.com/minz/minzc/pkg/mir2"

// This private serial-test hook exists only on the experimental branch.
// It is never copied into production or exposed through Options or CLI input.
var normalDiamondExperimentHook func(*mir2.Module) error
