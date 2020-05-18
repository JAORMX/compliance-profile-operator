package profileparser

import (
	"fmt"
	cmpv1alpha1 "github.com/JAORMX/compliance-profile-operator/pkg/apis/compliance/v1alpha1"
	"github.com/JAORMX/compliance-profile-operator/pkg/xccdf"
	"github.com/subchen/go-xmldom"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("profileparser")

type ParserConfig struct {
	DataStreamPath   string
	ProfileBundleKey types.NamespacedName
	Client           runtimeclient.Client
	Scheme           *k8sruntime.Scheme
}

func LogAndReturnError(errormsg string) error {
	log.Info(errormsg)
	return fmt.Errorf(errormsg)
}

func getVariableType(varNode *xmldom.Node) cmpv1alpha1.VariableType {
	typeAttr := varNode.GetAttribute("type")
	if typeAttr == nil {
		return cmpv1alpha1.VarTypeString
	}

	switch typeAttr.Value {
	case "string":
		return cmpv1alpha1.VarTypeString
	case "number":
		return cmpv1alpha1.VarTypeNumber
	case "boolean":
		return cmpv1alpha1.VarTypeBool
	}

	return cmpv1alpha1.VarTypeString
}

func ParseVariablesAndDo(contentDom *xmldom.Document, pcfg *ParserConfig, action func(v *cmpv1alpha1.Variable) error) error {
	varObjs := contentDom.Root.Query("//Value")
	for _, varObj := range varObjs {
		hidden := varObj.GetAttributeValue("hidden")
		if hidden == "true" {
			// this is typically used for functions
			continue
		}

		id := varObj.GetAttributeValue("id")
		log.Info("Found variable", "id", id)

		if id == "" {
			return LogAndReturnError("no id in variable")
		}
		title := varObj.FindOneByName("title")
		if title == nil {
			return LogAndReturnError("no title in variable")
		}

		v := cmpv1alpha1.Variable{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Variable",
				APIVersion: cmpv1alpha1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      xccdf.GetVariableNameFromID(id),
				Namespace: pcfg.ProfileBundleKey.Namespace,
			},
			ID:    id,
			Title: title.Text,
		}

		description := varObj.FindOneByName("description")
		if description != nil {
			v.Description = description.Text
		}

		v.Type = getVariableType(varObj)

		// extract the value and optionally the allowed value list
		err := parseVarValues(varObj, &v)
		if err != nil {
			log.Error(err, "couldn't set variable value")
			// We continue even if there's an error.
			continue
		}

		err = action(&v)
		if err != nil {
			log.Error(err, "couldn't execute action for variable")
			// We continue even if there's an error.
			continue
		}
	}
	return nil
}

func parseVarValues(varNode *xmldom.Node, v *cmpv1alpha1.Variable) error {
	for _, val := range varNode.FindByName("value") {
		selector := val.GetAttribute("selector")
		if selector != nil {
			// this is an enum choice
			v.Selections = append(v.Selections, cmpv1alpha1.ValueSelection{
				Description: selector.Value,
				Value:       val.Text,
			})
			continue
		}

		// this is the default value
		if v.Value != "" {
			return fmt.Errorf("attempting to set multiple values for variable %s; already had %s", v.ID, v.Value)
		}
		v.Value = val.Text
	}

	return nil
}
