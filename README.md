compliance-profile-operator
===========================

This is an operator which is in charge of managing compliance profiles. It
builds upon OpenSCAP profiles in order to do this.

Operands
--------

### Profile Bundle

A **ProfileBundle** object is a wrapper on an OpenSCAP datastream, this will
refer to a container image that packages an XCCDF file[1].


Example:

```
apiVersion: compliance.openshift.io/v1alpha1
kind: ProfileBundle
metadata:
  name: example-profilebundle
spec:
  contentImage: quay.io/jhrozek/ocp4-openscap-content:latest
  contentFile: ssg-ocp4-ds.xml
status:
  dataStreamStatus: VALID
```

Where:

* **spec.contentImage**: Contains a path to a container image to take into use
* **spec.contentFile**: is the path to access the datastream file whithin the
  image.
* **status.dataStreamStatus**: Will show the status of the datastream. e.g.
  whether it's usable or not (valid or invalid). If invalid, an error message
  will also appear in the status as the **status.errorMessage** key.

### Profile

A **Profile** is an object that represents a profile itself. Which is, un turn,
a set of rules that'll be checked for in a system.

Profiles will be creates by the operator itself and are not meant to be created
by administrators, these are derived from the **ProfileBundle** object.

Example:

```
apiVersion: compliance.openshift.io/v1alpha1
kind: Profile
metadata:
  name: moderate
title: NIST 800-53 Moderate-Impact Baseline for Red Hat Enterprise Linux CoreOS
description: |-
  This compliance profile reflects the core set of Moderate-Impact Baseline
  ...
id: xccdf_org.ssgproject.content_profile_moderate
rules:
- file_permissions_node_config
- account_disable_post_pw_expiration
```
Where:

* **title**: is the human-readable title of the profile
* **description**: is the more verbose description of the profile.
* **id**: it’s the ID from the xccdf document. This will help folks using raw
  (openscap) tools find the appropriate profile
* **rules**: contains the list of checks to be done on the system using this
  profile. This can be gotten by listing the xccdf-1.2:Rule instances in the
  profile from the datastream XML file.
* **values**: contains the variables that are set for the profile. These can
  be gotten from the xccdf-1.2:Value objects within the profile, which already
  give us information about the data type, the default value, and the current
  value that’s set for that variable.

TODO
----

* Detect if the container image path is valid: We need to verify whether the
  provided path is accessible, and if it isn't, persist that in the
  **ProfileBundle**'s status (see
  `pkg/controller/profilebundle/profilebundle_controller.go`)

* Use dedicated service account for **profileparser**: Currently the
  profile parser workload is using the same service account than the
  operator. This is not ideal as we really want to use a service account with
  less permissions. There already is a `profileparser` service account
  available (from the `deploy/service_account.yaml` file), however, using it
  causes an unkown issue in the workload. RBAC problems are suspected.

References
----------

[1] (https://csrc.nist.gov/CSRC/media/Publications/nistir/7275/rev-4/final/documents/nistir-7275r4_updated-march-2012_clean.pdf)
