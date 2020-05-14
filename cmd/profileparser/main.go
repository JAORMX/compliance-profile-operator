package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"github.com/JAORMX/compliance-profile-operator/pkg/profileparser"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	"github.com/spf13/pflag"
	"github.com/subchen/go-xmldom"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cmpv1alpha1 "github.com/JAORMX/compliance-profile-operator/pkg/apis/compliance/v1alpha1"
	"github.com/JAORMX/compliance-profile-operator/pkg/xccdf"
	"github.com/JAORMX/compliance-profile-operator/version"
)

var log = logf.Log.WithName("profileparser")

const (
	machineConfigFixType = "urn:xccdf:fix:script:ignition"
	kubernetesFixType    = "urn:xccdf:fix:script:kubernetes"
)

// XMLDocument is a wrapper that keeps the interface XML-parser-agnostic
type XMLDocument struct {
	*xmldom.Document
}

func printVersion() {
	log.Info(fmt.Sprintf("Operator Version: %s", version.Version))
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
}

func assertNotEmpty(param, paramName string) {
	if param == "" {
		log.Info("This cli parameter can't be empty", "parameter", paramName)
		os.Exit(1)
	}
}

func newParserConfig() *profileparser.ParserConfig {
	pcfg := profileparser.ParserConfig{}

	// Add the zap logger flag set to the CLI. The flag set must
	// be added before calling pflag.Parse().
	pflag.CommandLine.AddFlagSet(zap.FlagSet())

	// Add flags registered by imported packages (e.g. glog and
	// controller-runtime)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	pflag.StringVar(&pcfg.DataStreamPath, "ds-path", "/content/ssg-ocp4-ds.xml", "Path to the datastream xml file")
	pflag.StringVar(&pcfg.ProfileBundleKey.Name, "profile-bundle-name", "", "Name of the ProfileBundle object")
	pflag.StringVar(&pcfg.ProfileBundleKey.Namespace, "profile-bundle-namespace", "", "Namespace of the ProfileBundle object")

	pflag.Parse()

	logf.SetLogger(zap.Logger())

	printVersion()

	assertNotEmpty(pcfg.ProfileBundleKey.Name, "profile-bundle-name")
	assertNotEmpty(pcfg.ProfileBundleKey.Namespace, "profile-bundle-namespace")

	pcfg.Scheme = getK8sScheme()
	pcfg.Client = getK8sClient(pcfg.Scheme)

	return &pcfg
}

// The scheme registers the relevant objects into the k8s client
func getK8sScheme() *k8sruntime.Scheme {
	scheme := k8sruntime.NewScheme()

	scheme.AddKnownTypes(cmpv1alpha1.SchemeGroupVersion,
		&cmpv1alpha1.ProfileBundle{})
	scheme.AddKnownTypes(cmpv1alpha1.SchemeGroupVersion,
		&cmpv1alpha1.ProfileList{})
	scheme.AddKnownTypes(cmpv1alpha1.SchemeGroupVersion,
		&cmpv1alpha1.Profile{})
	scheme.AddKnownTypes(cmpv1alpha1.SchemeGroupVersion,
		&cmpv1alpha1.RuleList{})
	scheme.AddKnownTypes(cmpv1alpha1.SchemeGroupVersion,
		&cmpv1alpha1.Rule{})
	scheme.AddKnownTypes(cmpv1alpha1.SchemeGroupVersion,
		&cmpv1alpha1.VariableList{})
	scheme.AddKnownTypes(cmpv1alpha1.SchemeGroupVersion,
		&cmpv1alpha1.Variable{})
	scheme.AddKnownTypes(cmpv1alpha1.SchemeGroupVersion,
		&metav1.CreateOptions{})
	scheme.AddKnownTypes(cmpv1alpha1.SchemeGroupVersion,
		&metav1.UpdateOptions{})
	return scheme
}

// The client allows us to create k8s objects
func getK8sClient(scheme *k8sruntime.Scheme) runtimeclient.Client {
	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	client, err := runtimeclient.New(cfg, runtimeclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}
	return client
}

func getProfileBundle(pcfg *profileparser.ParserConfig) (*cmpv1alpha1.ProfileBundle, error) {
	pb := cmpv1alpha1.ProfileBundle{}

	err := pcfg.Client.Get(context.TODO(), pcfg.ProfileBundleKey, &pb)
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	return &pb, nil
}

func readContent(filename string) (*os.File, error) {
	// gosec complains that the file is passed through an evironment variable. But
	// this is not a security issue because none of the files are user-provided
	cleanFileName := filepath.Clean(filename)
	// #nosec G304
	return os.Open(cleanFileName)
}

func parseProfilesAndDo(contentDom *xmldom.Document, pcfg *profileparser.ParserConfig, action func(p *cmpv1alpha1.Profile) error) error {
	profileObjs := contentDom.Root.Query("//Profile")
	for _, profileObj := range profileObjs {
		id := profileObj.GetAttributeValue("id")
		if id == "" {
			return profileparser.LogAndReturnError("no id in profile")
		}
		title := profileObj.FindOneByName("title")
		if title == nil {
			return profileparser.LogAndReturnError("no title in profile")
		}
		description := profileObj.FindOneByName("description")
		if description == nil {
			return profileparser.LogAndReturnError("no description in profile")
		}
		log.Info("Found profile", "id", id)

		ruleObjs := profileObj.FindByName("select")
		selectedrules := []cmpv1alpha1.ProfileRule{}
		for _, ruleObj := range ruleObjs {
			idref := ruleObj.GetAttributeValue("idref")
			if idref == "" {
				log.Info("no idref in rule")
				continue
			}
			selected := ruleObj.GetAttributeValue("selected")
			if selected == "true" {
				ruleName := getPrefixedName(pcfg.ProfileBundleKey.Name, xccdf.GetRuleNameFromID(idref))
				selectedrules = append(selectedrules, cmpv1alpha1.NewProfileRule(ruleName))
			}
		}

		selectedvalues := []cmpv1alpha1.ProfileValue{}
		valueObjs := profileObj.FindByName("set-value")
		for _, valueObj := range valueObjs {
			idref := valueObj.GetAttributeValue("idref")
			if idref == "" {
				log.Info("no idref in rule")
				continue
			}
			selectedvalues = append(selectedvalues, cmpv1alpha1.ProfileValue(idref))
		}

		p := cmpv1alpha1.Profile{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Profile",
				APIVersion: cmpv1alpha1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      xccdf.GetProfileNameFromID(id),
				Namespace: pcfg.ProfileBundleKey.Namespace,
			},
			ID:          id,
			Title:       title.Text,
			Description: description.Text,
			Rules:       selectedrules,
			Values:      selectedvalues,
		}
		err := action(&p)
		if err != nil {
			log.Error(err, "couldn't execute action")
			return err
		}
	}

	return nil
}

func parseRulesAndDo(contentDom *xmldom.Document, pcfg *profileparser.ParserConfig, action func(p *cmpv1alpha1.Rule) error) error {
	ruleObjs := contentDom.Root.Query("//Rule")
	for _, ruleObj := range ruleObjs {
		id := ruleObj.GetAttributeValue("id")
		if id == "" {
			return profileparser.LogAndReturnError("no id in rule")
		}
		title := ruleObj.FindOneByName("title")
		if title == nil {
			return profileparser.LogAndReturnError("no title in rule")
		}
		log.Info("Found rule", "id", id)

		description := ruleObj.FindOneByName("description")
		rationale := ruleObj.FindOneByName("rationale")
		warning := ruleObj.FindOneByName("warning")
		severity := ruleObj.FindOneByName("severity")

		fixes := []cmpv1alpha1.FixDefinition{}
		foundPlatformMap := make(map[string]bool)
		fixNodeObjs := ruleObj.FindByName("fix")
		for _, fixNodeObj := range fixNodeObjs {
			if !isRelevantFix(fixNodeObj) {
				continue
			}
			platform := fixNodeObj.GetAttributeValue("platform")
			if foundPlatformMap[platform] {
				// We already have a remediation for this platform
				continue
			}

			rawFixReader := strings.NewReader(fixNodeObj.Text)
			fixKubeObj, err := readObjFromYAML(rawFixReader)
			if err != nil {
				log.Info("Couldn't parse Kubernetes object from fix")
				continue
			}

			disruption := fixNodeObj.GetAttributeValue("disruption")

			newFix := cmpv1alpha1.FixDefinition{
				Disruption: disruption,
				Platform:   platform,
				FixObject:  fixKubeObj,
			}
			fixes = append(fixes, newFix)
			foundPlatformMap[platform] = true
		}

		p := cmpv1alpha1.Rule{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Rule",
				APIVersion: cmpv1alpha1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      xccdf.GetRuleNameFromID(id),
				Namespace: pcfg.ProfileBundleKey.Namespace,
			},
			ID:             id,
			Title:          title.Text,
			AvailableFixes: nil,
		}
		if description != nil {
			p.Description = description.Text
		}
		if rationale != nil {
			p.Rationale = rationale.Text
		}
		if warning != nil {
			p.Warning = warning.Text
		}
		if severity != nil {
			p.Severity = severity.Text
		}
		if len(fixes) > 0 {
			p.AvailableFixes = fixes
		}
		err := action(&p)
		if err != nil {
			log.Error(err, "couldn't execute action for rule")
			// We continue even if there's an error.
		}
	}

	return nil
}

// Reads a YAML file and returns an unstructured object from it. This object
// can be taken into use by the dynamic client
func readObjFromYAML(r io.Reader) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	dec := k8syaml.NewYAMLToJSONDecoder(r)
	err := dec.Decode(obj)
	return obj, err
}

func isRelevantFix(fix *xmldom.Node) bool {
	if fix.GetAttributeValue("system") == machineConfigFixType {
		return true
	}
	if fix.GetAttributeValue("system") == kubernetesFixType {
		return true
	}
	return false
}

func getPrefixedName(pbName, objName string) string {
	return pbName + "-" + objName
}

// updateProfileBundleStatus updates the status of the given ProfileBundle. If
// the given error is nil, the status will be valid, else it'll be invalid
func updateProfileBundleStatus(pcfg *profileparser.ParserConfig, pb *cmpv1alpha1.ProfileBundle, err error) {
	if err != nil {
		// Never update a fetched object, always just a copy
		pbCopy := pb.DeepCopy()
		pbCopy.Status.DataStreamStatus = cmpv1alpha1.DataStreamInvalid
		pbCopy.Status.ErrorMessage = err.Error()
		err = pcfg.Client.Status().Update(context.TODO(), pbCopy)
		if err != nil {
			log.Error(err, "Couldn't update ProfileBundle status")
			os.Exit(1)
		}
	} else {
		// Never update a fetched object, always just a copy
		pbCopy := pb.DeepCopy()
		pbCopy.Status.DataStreamStatus = cmpv1alpha1.DataStreamValid
		err = pcfg.Client.Status().Update(context.TODO(), pbCopy)
		if err != nil {
			log.Error(err, "Couldn't update ProfileBundle status")
			os.Exit(1)
		}
	}
}

func main() {
	pcfg := newParserConfig()

	pb, err := getProfileBundle(pcfg)
	if err != nil {
		log.Error(err, "Couldn't get ProfileBundle")

		os.Exit(1)
	}

	contentFile, err := readContent(pcfg.DataStreamPath)
	if err != nil {
		log.Error(err, "Couldn't read the content")
		updateProfileBundleStatus(pcfg, pb, fmt.Errorf("Couldn't read content file: %s", err))
		os.Exit(1)
	}
	// #nosec
	defer contentFile.Close()
	bufContentFile := bufio.NewReader(contentFile)
	contentDom, err := xmldom.Parse(bufContentFile)
	if err != nil {
		log.Error(err, "Couldn't read the content XML")
		updateProfileBundleStatus(pcfg, pb, fmt.Errorf("Couldn't read content XML: %s", err))
		os.Exit(1)
	}

	err = parseProfilesAndDo(contentDom, pcfg, func(p *cmpv1alpha1.Profile) error {
		pCopy := p.DeepCopy()
		profileName := pCopy.Name
		// overwrite name
		pCopy.SetName(getPrefixedName(pb.Name, profileName))

		if err := controllerutil.SetControllerReference(pb, pCopy, pcfg.Scheme); err != nil {
			return err
		}

		log.Info("Creating Profile", "Profile.name", p.Name)
		err := pcfg.Client.Create(context.TODO(), pCopy)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				log.Info("Profile already exists.", "Profile.Name", p.Name)
			} else {
				log.Error(err, "couldn't create profile")
				return err
			}
		}
		return nil
	})

	if err != nil {
		updateProfileBundleStatus(pcfg, pb, err)
		return
	}

	err = parseRulesAndDo(contentDom, pcfg, func(r *cmpv1alpha1.Rule) error {
		ruleName := r.Name
		// overwrite name
		r.SetName(getPrefixedName(pb.Name, ruleName))

		if err := controllerutil.SetControllerReference(pb, r, pcfg.Scheme); err != nil {
			return err
		}

		log.Info("Creating rule", "Rule.Name", r.Name)
		err := pcfg.Client.Create(context.TODO(), r)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				log.Info("Rule already exists.", "Rule.Name", r.Name)
			} else {
				log.Error(err, "couldn't create Rule")
				return err
			}
		}
		return nil
	})

	if err != nil {
		updateProfileBundleStatus(pcfg, pb, err)
		return
	}

	err = profileparser.ParseVariablesAndDo(contentDom, pcfg, func(v *cmpv1alpha1.Variable) error {
		varName := v.Name
		// overwrite name
		v.SetName(getPrefixedName(pb.Name, varName))

		if err := controllerutil.SetControllerReference(pb, v, pcfg.Scheme); err != nil {
			return err
		}

		log.Info("Creating variable", "Variable.Name", v.Name)
		err := pcfg.Client.Create(context.TODO(), v)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				log.Info("Variable already exists.", "Variable.Name", v.Name)
			} else {
				log.Error(err, "couldn't create Variable")
				return err
			}
		}
		return nil
	})

	// The err variable might be nil, this is fine, it'll just update the status
	// to valid
	updateProfileBundleStatus(pcfg, pb, err)
}
