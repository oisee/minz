package main

import (
	"testing"
)

func orderedByteCLIState(t *testing.T) {
	t.Helper()
	hirProfileTestState(t)
	oldOrdered, oldRegion, oldCopy, oldMixed, oldJoint := hirProfileOrderedByteFunctions, hirProfileRegionCompareSites, hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites, hirProfileJoint
	oldInvariant, oldSite, oldJointSites, oldWeighted, oldIndependent := hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules
	oldComposition := hirProfileMixedCopyRegionSites
	oldDir, oldFn, oldWork := hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload
	t.Cleanup(func() {
		hirProfileMixedCopyRegionSites = oldComposition
		hirProfileOrderedByteFunctions, hirProfileRegionCompareSites, hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites, hirProfileJoint = oldOrdered, oldRegion, oldCopy, oldMixed, oldJoint
		hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = oldInvariant, oldSite, oldJointSites, oldWeighted, oldIndependent
		hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload = oldDir, oldFn, oldWork
	})
	hirProfileOrderedByteFunctions, hirProfileJoint = true, true
	hirProfileRegionCompareSites, hirProfileMixedCopyRegionSites, hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites = false, false, false, false
	hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = false, false, false, false, false
	hirProfileAutoFunction = ""
}
func TestHIRProfileOrderedByteValidation(t *testing.T) {
	orderedByteCLIState(t)
	for _, name := range []string{"joint", "region", "composition", "copy", "mixed", "invariant", "site", "jointsites", "weighted", "independent", "auto", "workload", "restriction", "separate"} {
		t.Run(name, func(t *testing.T) {
			hirProfileAutoDirectory = "evidence"
			hirProfileAutoWorkload = "workload.json"
			hirProfileAutoFunction = ""
			hirProfile = hirProfileFlags{}
			hirProfileJoint = true
			hirProfileRegionCompareSites, hirProfileMixedCopyRegionSites = false, false
			hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites, hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = false, false, false, false, false, false, false
			switch name {
			case "joint":
				hirProfileJoint = false
			case "region":
				hirProfileRegionCompareSites = true
			case "composition":
				hirProfileMixedCopyRegionSites = true
			case "copy":
				hirProfileForwardCopySites = true
			case "mixed":
				hirProfileMixedCopyWeightedSites = true
			case "invariant":
				hirProfileInvariantSites = true
			case "site":
				hirProfileSiteRules = true
			case "jointsites":
				hirProfileJointSites = true
			case "weighted":
				hirProfileWeightedSites = true
			case "independent":
				hirProfileIndependentRules = true
			case "auto":
				hirProfileAutoDirectory = ""
			case "workload":
				hirProfileAutoWorkload = ""
			case "restriction":
				hirProfileAutoFunction = "read_word"
			case "separate":
				hirProfile.profile = "profile.json"
			}
			if validateHIRProfileAuto("ordered-byte.nanz") == nil {
				t.Fatal("incompatible ordered-byte mode accepted", name)
			}
		})
	}
	// Shared package globals are restored by the outer state helper; each sibling
	// validation mode also returns to a clean state before this successful check.
	hirProfileAutoDirectory, hirProfileAutoWorkload, hirProfileAutoFunction = "evidence", "workload.json", ""
	hirProfile = hirProfileFlags{}
	hirProfileJoint = true
	hirProfileRegionCompareSites, hirProfileMixedCopyRegionSites = false, false
	hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites, hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = false, false, false, false, false, false, false
	if err := validateHIRProfileAuto("ordered-byte.nanz"); err != nil {
		t.Fatal("valid ordered-byte mode refused", err)
	}
}
