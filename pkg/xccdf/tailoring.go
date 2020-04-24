package xccdf

import (
	"encoding/xml"
	"path/filepath"
	"strings"

	cmpv1alpha1 "github.com/JAORMX/compliance-profile-operator/pkg/apis/compliance/v1alpha1"
)

const (
	// XMLHeader is the header for the XML doc
	XMLHeader       string = `<?xml version="1.0" encoding="UTF-8"?>`
	profileIDPrefix string = "xccdf_org.ssgproject.content_profile_"
	ruleIDPrefix    string = "xccdf_org.ssgproject.content_rule_"
)

type TailoringElement struct {
	XMLName   xml.Name `xml:"xccdf-1.2:Tailoring"`
	ID        string   `xml:"id,attr"`
	Benchmark BenchmarkElement
	// TODO(jaosorior): Add time attribute
	// Time TimeElement
	Profile ProfileElement
	// TODO(jaosorior): Add signature capabilities
	// Signature SignatureElement
}

type BenchmarkElement struct {
	XMLName xml.Name `xml:"xccdf-1.2:benchmark"`
	Href    string   `xml:"href,attr"`
}

type ProfileElement struct {
	XMLName     xml.Name `xml:"xccdf-1.2:Profile"`
	ID          string   `xml:"id,attr"`
	Extends     string   `xml:"extends,attr"`
	Title       string   `xml:"xccdf-1.2:title,omitempty"`
	Description string   `xml:"xccdf-1.2:description,omitempty"`
	Selections  []SelectElement
}

type SelectElement struct {
	XMLName  xml.Name `xml:"xccdf-1.2:select"`
	IDRef    string   `xml:"idref,attr"`
	Selected bool     `xml:"selected,attr"`
}

// GetXCCDFProfileID gets a profile xccdf ID from the TailoredProfile object
func GetXCCDFProfileID(tp *cmpv1alpha1.TailoredProfile) string {
	return profileIDPrefix + tp.Name
}

// GetXCCDFRuleID gets a rule xccdf ID from the rule name
func GetXCCDFRuleID(selectionName string) string {
	return ruleIDPrefix + selectionName
}

// GetProfileNameFromID gets a profile name from the xccdf ID
func GetProfileNameFromID(id string) string {
	return strings.TrimPrefix(id, profileIDPrefix)
}

// GetRuleNameFromID gets a rule name from the xccdf ID
func GetRuleNameFromID(id string) string {
	return strings.TrimPrefix(id, ruleIDPrefix)
}

func getSelectElementFromCRRule(selection cmpv1alpha1.RuleReferenceSpec, enable bool) SelectElement {
	return SelectElement{
		IDRef:    GetXCCDFRuleID(selection.Name),
		Selected: enable,
	}
}

func getSelections(tp *cmpv1alpha1.TailoredProfile) []SelectElement {
	selections := []SelectElement{}
	for _, selection := range tp.Spec.EnableRules {
		selections = append(selections, getSelectElementFromCRRule(selection, true))
	}

	for _, selection := range tp.Spec.DisableRules {
		selections = append(selections, getSelectElementFromCRRule(selection, false))
	}
	return selections
}

// TailoredProfileToXML gets an XML string from a TailoredProfile and the corresponding Profile
func TailoredProfileToXML(tp *cmpv1alpha1.TailoredProfile, p *cmpv1alpha1.Profile, pb *cmpv1alpha1.ProfileBundle) (string, error) {
	tailoring := TailoringElement{
		ID: "xccdf_scap-workbench_tailoring_default",
		Benchmark: BenchmarkElement{
			// NOTE(jaosorior): Both this operator and the compliance-operator
			// assume the content will be mounted on a "content/" directory
			Href: filepath.Join("/content", pb.Spec.ContentFile),
		},
		Profile: ProfileElement{
			ID:          GetXCCDFProfileID(tp),
			Extends:     p.ID,
			Title:       tp.Spec.Title,
			Description: tp.Spec.Description,
			Selections:  getSelections(tp),
		},
	}

	output, err := xml.MarshalIndent(tailoring, "", "  ")
	if err != nil {
		return "", err
	}
	return XMLHeader + "\n" + string(output), nil
}
