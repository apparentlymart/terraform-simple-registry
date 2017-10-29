# Terraform Simple Registry

This repository contains simple implementations of the Terraform registry
protocols.

This package is designed with the principle of "small pieces loosely joined":
it deals only with the Terraform protocols and expects other concerns to be
dealt with via complementary software. For example:

* Each of the registry services is provided as a separate program, allowing the
  user to decide which to use and how to deploy them.

* Authentication is not directly integrated, but can be applied by e.g. putting
  nginx in front of this program's server and configuring it to verify
  _JSON Web Tokens_, or similar.

* These programs provide read-only access to data available in the local
  filesystem. This data can either be placed manually or updated automatically
  by separate software in response to hooks from a remote Git repository, etc.

* A simple, static configuration system is used, so these files can either be
  hand-edited or generated automatically by some other external program.

Delegating all but the core concerns to other software gives users the
flexibility to customize their deployment and integrate these programs with
other systems and processes.

This program is not a HashiCorp product and is not directly supported by
the Terraform team at HashiCorp.

An "out-of-the-box" private registry solution, with support from HashiCorp,
is available as part of [Terraform Enterprise](https://www.hashicorp.com/products/terraform).
This implementation integrates well with other Terraform Enterprise
features.
