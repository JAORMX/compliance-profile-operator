package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
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
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cmpv1alpha1 "github.com/JAORMX/compliance-profile-operator/pkg/apis/compliance/v1alpha1"
	"github.com/JAORMX/compliance-profile-operator/version"
)

var log = logf.Log.WithName("profileparser")

type parserConfig struct {
	dataStreamPath   string
	profileBundleKey types.NamespacedName
	client           runtimeclient.Client
	scheme           *k8sruntime.Scheme
}

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

func newParserConfig() *parserConfig {
	pcfg := parserConfig{}

	// Add the zap logger flag set to the CLI. The flag set must
	// be added before calling pflag.Parse().
	pflag.CommandLine.AddFlagSet(zap.FlagSet())

	// Add flags registered by imported packages (e.g. glog and
	// controller-runtime)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	pflag.StringVar(&pcfg.dataStreamPath, "ds-path", "/content/ssg-ocp4-ds.xml", "Path to the datastream xml file")
	pflag.StringVar(&pcfg.profileBundleKey.Name, "profile-bundle-name", "", "Name of the ProfileBundle object")
	pflag.StringVar(&pcfg.profileBundleKey.Namespace, "profile-bundle-namespace", "", "Namespace of the ProfileBundle object")

	pflag.Parse()

	logf.SetLogger(zap.Logger())

	printVersion()

	assertNotEmpty(pcfg.profileBundleKey.Name, "profile-bundle-name")
	assertNotEmpty(pcfg.profileBundleKey.Namespace, "profile-bundle-namespace")

	pcfg.scheme = getK8sScheme()
	pcfg.client = getK8sClient(pcfg.scheme)

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

func getProfileBundle(pcfg *parserConfig) (*cmpv1alpha1.ProfileBundle, error) {
	pb := cmpv1alpha1.ProfileBundle{}

	err := pcfg.client.Get(context.TODO(), pcfg.profileBundleKey, &pb)
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

func parseProfilesAndDo(dsReader io.Reader, pcfg *parserConfig, action func(p *cmpv1alpha1.Profile) error) error {
	resultsDom, err := xmldom.Parse(dsReader)
	if err != nil {
		return err
	}
	profileObjs := resultsDom.Root.Query("//Profile")
	for _, profileObj := range profileObjs {
		id := profileObj.GetAttributeValue("id")
		if id == "" {
			errormsg := "no id in profile"
			log.Info(errormsg)
			return fmt.Errorf(errormsg)
		}
		title := profileObj.FindOneByName("title")
		if title == nil {
			errormsg := "no title in profile"
			log.Info(errormsg)
			return fmt.Errorf(errormsg)
		}
		description := profileObj.FindOneByName("description")
		if description == nil {
			errormsg := "no description in profile"
			log.Info(errormsg)
			return fmt.Errorf(errormsg)
		}
		log.Info("Found profile", "id", id)

		ruleObjs := profileObj.Query("//select")
		selectedrules := []cmpv1alpha1.ProfileRule{}
		for _, ruleObj := range ruleObjs {
			idref := ruleObj.GetAttributeValue("idref")
			if idref == "" {
				log.Info("no idref in rule")
				continue
			}
			selected := ruleObj.GetAttributeValue("selected")
			if selected == "true" {
				selectedrules = append(selectedrules, cmpv1alpha1.NewProfileRule(getRuleNameFromID(idref)))
			}
		}

		selectedvalues := []cmpv1alpha1.ProfileValue{}
		valueObjs := profileObj.Query("//set-value")
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
				Name:      getProfileNameFromID(id),
				Namespace: pcfg.profileBundleKey.Namespace,
			},
			ID:          id,
			Title:       title.Text,
			Description: description.Text,
			Rules:       selectedrules,
			Values:      selectedvalues,
		}
		err = action(&p)
		if err != nil {
			log.Error(err, "couldn't execute action")
			return err
		}
	}

	return nil
}

func getProfileNameFromID(id string) string {
	const idPrefix = "xccdf_org.ssgproject.content_profile_"
	return strings.TrimPrefix(id, idPrefix)
}

func getRuleNameFromID(id string) string {
	const idPrefix = "xccdf_org.ssgproject.content_rule_"
	return strings.TrimPrefix(id, idPrefix)
}
func getPrefixedProfileName(pb *cmpv1alpha1.ProfileBundle, profileName string) string {
	return pb.Name + "-" + profileName
}

// updateProfileBundleStatus updates the status of the given ProfileBundle. If
// the given error is nil, the status will be valid, else it'll be invalid
func updateProfileBundleStatus(pcfg *parserConfig, pb *cmpv1alpha1.ProfileBundle, err error) {
	if err != nil {
		// Never update a fetched object, always just a copy
		pbCopy := pb.DeepCopy()
		pbCopy.Status.DataStreamStatus = cmpv1alpha1.DataStreamInvalid
		pbCopy.Status.ErrorMessage = err.Error()
		err = pcfg.client.Status().Update(context.TODO(), pbCopy)
		if err != nil {
			log.Error(err, "Couldn't update ProfileBundle status")
			os.Exit(1)
		}
	} else {
		// Never update a fetched object, always just a copy
		pbCopy := pb.DeepCopy()
		pbCopy.Status.DataStreamStatus = cmpv1alpha1.DataStreamValid
		err = pcfg.client.Status().Update(context.TODO(), pbCopy)
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

	contentFile, err := readContent(pcfg.dataStreamPath)
	if err != nil {
		log.Error(err, "Couldn't read the content")
		updateProfileBundleStatus(pcfg, pb, fmt.Errorf("Couldn't read content file: %s", err))
		os.Exit(1)
	}
	// #nosec
	defer contentFile.Close()
	bufContentFile := bufio.NewReader(contentFile)

	err = parseProfilesAndDo(bufContentFile, pcfg, func(p *cmpv1alpha1.Profile) error {
		pCopy := p.DeepCopy()
		profileName := pCopy.Name
		// overwrite name
		pCopy.SetName(getPrefixedProfileName(pb, profileName))

		if err := controllerutil.SetControllerReference(pb, pCopy, pcfg.scheme); err != nil {
			return err
		}

		log.Info("Creating profile profile", "name", p.Name)
		err := pcfg.client.Create(context.TODO(), pCopy)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				log.Info("Profile already exists.", "name", p.Name)
			} else {
				log.Error(err, "couldn't create profile")
				return err
			}
		}
		return nil
	})

	// The err variable might be nil, this is fine, it'll just update the status
	// to valid
	updateProfileBundleStatus(pcfg, pb, err)
}
