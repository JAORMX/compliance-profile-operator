Developing
==========

The `Makefile` already has a lot of workflows that aim to aide developers in
getting on-board the project.

The following command will list the available options:

```
make help
```

Making changes to the CRDs
--------------------------

The Custom Resources are defined in `pkg/apis/compliance/v1alpha1/*_types.go`.
Any changes to those files end up getting reflected in the CRD objects. The CRD
objects themselves are in `deploy/crds/*_crd.yaml`. One can auto-generate these
CRDs by doing:

```
make generate
```

Note that **operator-sdk** is required. If you don't have it, the Makefile will
try to download it for you.

Deploying
---------

The makefile contains a handy command to deploy the operator using the
manifests that exist in this repo:

```
make deploy
```


Pushing images to the cluster
-----------------------------

You can build and push the images you have locally to the OpenShift cluster by
using the following:

```
make image-to-cluster
```

Deploying a locally built instance
----------------------------------

If you want to test local changes to the operator, the makefile can also help
with this!

```
make deploy-local
```

This will build the container images, push them to the OpenShift cluster you're
using, and then try to deploy the operator.

If you already pushed the images to the cluster and don't wish to build the
images every time you try to deploy, you can do:

```
export SKIP_CONTAINER_PUSH=true
```
