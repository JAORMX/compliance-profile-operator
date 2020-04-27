package xccdf

import (
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strings"

	cmpv1alpha1 "github.com/JAORMX/compliance-profile-operator/pkg/apis/compliance/v1alpha1"
)

const (
	// XMLHeader is the header for the XML doc
	XMLHeader       string = `<?xml version="1.0" encoding="UTF-8"?>`
	profileIDPrefix string = "xccdf_org.ssgproject.content_profile_"
	ruleIDPrefix    string = "xccdf_org.ssgproject.content_rule_"
	// XCCDFNamespace is the XCCDF namespace of this project. Per the XCCDF
	// specification, this assiciates the content with the author
	XCCDFNamespace string = "compliance.openshift.io"
	XCCDFURI       string = "http://checklists.nist.gov/xccdf/1.2"
)

type TailoringElement struct {
	XMLName         xml.Name `xml:"xccdf-1.2:Tailoring"`
	XMLNamespaceURI string   `xml:"xmlns:xccdf,attr"`
	ID              string   `xml:"id,attr"`
	Benchmark       BenchmarkElement
	// TODO(jaosorior): Add version attribute
	// Version versionElement
	Profile ProfileElement
	// TODO(jaosorior): Add signature capabilities
	// Signature SignatureElement
}

type BenchmarkElement struct {
	XMLName xml.Name `xml:"xccdf-1.2:benchmark"`
	Href    string   `xml:"href,attr"`
}

type ProfileElement struct {
	XMLName     xml.Name                   `xml:"xccdf-1.2:Profile"`
	ID          string                     `xml:"id,attr"`
	Extends     string                     `xml:"extends,attr"`
	Title       *TitleOrDescriptionElement `xml:"xccdf-1.2:title,omitempty"`
	Description *TitleOrDescriptionElement `xml:"xccdf-1.2:description,omitempty"`
	Selections  []SelectElement
}

type TitleOrDescriptionElement struct {
	Override bool   `xml:"override,attr"`
	Value    string `xml:",chardata"`
}

type SelectElement struct {
	XMLName  xml.Name `xml:"xccdf-1.2:select"`
	IDRef    string   `xml:"idref,attr"`
	Selected bool     `xml:"selected,attr"`
}

// GetXCCDFProfileID gets a profile xccdf ID from the TailoredProfile object
func GetXCCDFProfileID(tp *cmpv1alpha1.TailoredProfile) string {
	return fmt.Sprintf("xccdf_%s_profile_%s", XCCDFNamespace, tp.Name)
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

func getTailoringID(tp *cmpv1alpha1.TailoredProfile) string {
	return fmt.Sprintf("xccdf_%s_tailoring_%s", XCCDFNamespace, tp.Name)
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
		XMLNamespaceURI: XCCDFURI,
		ID:              getTailoringID(tp),
		Benchmark: BenchmarkElement{
			// NOTE(jaosorior): Both this operator and the compliance-operator
			// assume the content will be mounted on a "content/" directory
			Href: filepath.Join("/content", pb.Spec.ContentFile),
		},
		Profile: ProfileElement{
			ID:         GetXCCDFProfileID(tp),
			Extends:    p.ID,
			Selections: getSelections(tp),
		},
	}
	if tp.Spec.Title != "" {
		tailoring.Profile.Title = &TitleOrDescriptionElement{
			Override: true,
			Value:    tp.Spec.Title,
		}
	}
	if tp.Spec.Description != "" {
		tailoring.Profile.Description = &TitleOrDescriptionElement{
			Override: true,
			Value:    tp.Spec.Description,
		}
	}

	output, err := xml.MarshalIndent(tailoring, "", "  ")
	if err != nil {
		return "", err
	}
	return XMLHeader + "\n" + string(output), nil
}
