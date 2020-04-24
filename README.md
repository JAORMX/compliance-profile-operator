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
* **spec.contentFile**: is the path to access the datastream file within the
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

## TailoredProfile

A **TailoredProfile** is an object that represents changes that need to be done
for a specific profile. Given that profiles are meant to be immutable, these
changes need a specific object (which is this one).

Example:

```
kind: TailoredProfile
apiVersion: compliance.openshift.io/v1alpha1
metadata:
  name: moderate-custom
spec:
  extends: moderate
  title: | 
    NIST 800-53 Moderate-Impact Baseline for Red Hat Enterprise Linux
    CoreOS customized for this deployment
  description: |
    This compliance profile reflects the core set of Moderate-Impact
    Baseline configuration settings for deployment of Red Hat
    Enterprise
    …
  enableRules:
    - ruleName: chronyd_client_only
      rationale: We really need to enable this
  disableRules:
    - ruleName: chronyd_no_chronyc_network
      rationale: This doesn’t apply to my cluster
  variables:
    - var_password_pam_difok: 5
status:
  id: xccdf_org.ssgproject.content_profile_ocp4-moderate-custom
  tailoringConfigMap:
    name: moderate-custom-tp
    namespace: this-namespace
```

Where:

* **name**: Is simply the name of the tailored profile, however, this name will
  be taken into account when generating the tailoring file and the xccdf ID
  will be derived from this.
* **spec.extends**: should be an existing Profile object which we want to
  extend as part of this tailoring. From here we’ll derive several attributes
* **spec.title**: the new title for this customized profile. If this isn’t
  specified, the same title as the profile that’s being extended will be used.
* **spec.description**: the new description for this customized profile. If
  this isn’t specified the same description from the profile that’s being
  extended will be used.
* **spec.enableRules**: Checks to be enabled (if they were disabled before) as
  part of this customized profile.
* **disableRules**: Checks to be disabled (if they were enabled before) as part
  of this customized profile.
* **spec.variables**: Set values of variables from the profile.
* **status.id**: This is the xccdf ID to take the profile into use with the
  oscap tool.
* **status.tailoringConfigMap**: Once a tailored profile has been processed,
  the XML will be outputted as a ConfigMap. This ConfigMap will simply be the
  raw XML generated for the tailoring and can be taken into use directly.

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
  causes an unknown issue in the workload. RBAC problems are suspected.

* Implement **variables** in the **TailoredProfile** object.

* Implement value specifications in the tailored profiles. Currently only
  selections are handled.

* Create default ProfileBundle(s) when the operator starts. This would be
  useful in the sense that we would already have an OpenShift bundle and
  profiles created from it as a default.

* Make profile parser workload overwrite existing profiles. In cases where the
  profile might have been modified locally, it would be good to get the
  profileparser workload to be able to update the profiles so they're back to a
  good state.

References
----------

[1] (https://csrc.nist.gov/CSRC/media/Publications/nistir/7275/rev-4/final/documents/nistir-7275r4_updated-march-2012_clean.pdf)
