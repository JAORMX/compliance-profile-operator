package profileparser

import (
	cmpv1alpha1 "github.com/JAORMX/compliance-profile-operator/pkg/apis/compliance/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/types"

	"github.com/subchen/go-xmldom"
)

func varHaveID(id string) gomegatypes.GomegaMatcher {
	return WithTransform(func(p cmpv1alpha1.Variable) string { return p.ID }, Equal(id))
}

func getVariableById(id string, varList []cmpv1alpha1.Variable) *cmpv1alpha1.Variable {
	for _, variable := range varList {
		if id == variable.ID {
			return &variable
		}
	}

	return nil
}

var _ = Describe("Testing parse variables", func() {
	var (
		pcfg       *ParserConfig
		contentDom *xmldom.Document
		err        error
		varList    []cmpv1alpha1.Variable
	)

	BeforeEach(func() {
		pcfg = &ParserConfig{
			DataStreamPath: "../../tests/data/ssg-ocp4-ds.xml",
			ProfileBundleKey: types.NamespacedName{
				Namespace: "test-namespace",
				Name:      "test-profile",
			},
			Client: nil, // not needed for a test
			Scheme: nil, // not needed for a test
		}

		contentDom, err = xmldom.ParseFile(pcfg.DataStreamPath)
		Expect(err).NotTo(HaveOccurred())

		varList = make([]cmpv1alpha1.Variable, 0)
		variableAdder := func(p *cmpv1alpha1.Variable) error {
			varList = append(varList, *p)
			return nil
		}

		err := ParseVariablesAndDo(contentDom, pcfg, variableAdder)
		Expect(err).To(BeNil())
	})

	Context("Some variables are parsed", func() {
		const expectedID = "xccdf_org.ssgproject.content_value_var_sshd_max_sessions"

		It("Contains the expected variable", func() {
			Expect(varList).Should(ContainElements(varHaveID(expectedID)))
		})
	})

	Context("Variables have the expected metadata", func() {
		var sshdPrivSepVar *cmpv1alpha1.Variable

		BeforeEach(func() {
			const expectedID = "xccdf_org.ssgproject.content_value_var_sshd_priv_separation"

			sshdPrivSepVar = getVariableById(expectedID, varList)
			Expect(sshdPrivSepVar).ToNot(BeNil())
		})

		It("Has the expected title", func() {
			const expectedTitle = "SSH Privilege Separation Setting"
			Expect(sshdPrivSepVar.Title).To(BeEquivalentTo(expectedTitle))
		})

		It("Has the expected description", func() {
			const expectedDescription = "Specify whether and how sshd separates privileges when handling incoming network connections."
			Expect(sshdPrivSepVar.Description).To(BeEquivalentTo(expectedDescription))
		})

		It("Has the expected selections", func() {
			Expect(sshdPrivSepVar.Selections).To(ConsistOf([]cmpv1alpha1.ValueSelection{
				{
					Description: "no",
					Value:       "no",
				},
				{
					Description: "yes",
					Value:       "yes",
				},
				{
					Description: "sandbox",
					Value:       "sandbox",
				},
			}))
		})

		It("Has the expected default value", func() {
			Expect(sshdPrivSepVar.Value).To(BeEquivalentTo("sandbox"))
		})

		It("Has the expected type", func() {
			Expect(sshdPrivSepVar.Type).To(BeEquivalentTo("string"))
		})
	})
})
