package common

import "os"

// ComplianceComponent defines the component ID
type ComplianceComponent uint

const (
	// PROFILEPARSER identifies the profileparser image
	PROFILEPARSER = iota
)

var componentDefaults = []struct {
	defaultImage string
	envVar       string
}{
	{"quay.io/compliance-profile-operator/profileparser:latest", "PROFILEPARSER_IMAGE"},
}

// GetComponentImage returns a full image pull spec for a given component
// based on the component type
func GetComponentImage(component ComplianceComponent) string {
	comp := componentDefaults[component]

	imageTag := os.Getenv(comp.envVar)
	if imageTag == "" {
		imageTag = comp.defaultImage
	}
	return imageTag
}
