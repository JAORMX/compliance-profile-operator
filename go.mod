module github.com/JAORMX/compliance-profile-operator

go 1.13

require (
	github.com/operator-framework/operator-sdk v0.17.1-0.20200421202908-c5877e2019ca
	github.com/securego/gosec v0.0.0-20200401082031-e946c8c39989
	github.com/spf13/pflag v1.0.5
	github.com/subchen/go-xmldom v1.1.2
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.5.2
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.17.4 // Required by prometheus-operator
)
