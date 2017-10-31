# Terraform Module Registry v1 Server

This directory contains a Go program that provides a minimal implementation
of Terraform's module registry protocol based on one or more local git
repositories.

It is part of [the "simple registry" suite of programs](../../) that provide
building blocks for deploying a local Terraform registry.

## Installation

This program is `go get`-able:

```
$ go get github.com/apparentlymart/terraform-simple-registry/cmd/terraform-modules-v1-server
```

In future precompiled binaries may be provided, but for now it's required that
you build from source using Go 1.9 or above.

## Theory of Operation

Consistent with the design goals of this suite, this server provides only
the basic module registry functionality and expects other concerns, such
as authentication, to be handled by other frontend servers like `nginx`, using
this server as a backend.

This server is configured with one or more modules, each of which is backed
by a git repository on the local filesystem. The server uses a tag naming
convention of `v` followed by a version string to determine which versions
are available for each module. When Terraform requests to download a particular
version, the server writes the contents of the relevant git commit into a
tar archive to send to the client.

The management of these local git repositories is left up to the user. For
simple deployments it may be reasonable to just manually clone some other
repositories and periodically sync then, or to `git push` from somewhere else
onto the server running this server. In more complex scenarios, you may wish
to use a separate program to respond to hooks on an upstream repository or in
a CI system and re-sync the local git repositories automatically.

## Usage

The program accepts one or more arguments which are all interpreted as either
configuration files directly or as directories containing potentially-multiple
configuration files.

Configuration files are in HCL format, unless they have a `.json` file extension
in which case they are interpreted as JSON-flavored HCL.

```
$ terraform-modules-v1-server /etc/terraform-registry/modules-v1.conf
```

The configuration file contents are described in the following section.

## Configuration File

The configuration file deals with three different concerns:

* The "friendly hostname" of the module registry, which it must know in order
  to produce canonical module source strings.
* The set of modules to publish and their associated git directories.
* One or more listener configurations, causing the server to listen for requests
  over either HTTP or FastCGI, with optional TLS.

The `hostname` top-level attribute specifies the registry's "friendly hostname".
This must match the hostname users will use in module source strings to install
from this registry:

```hcl
hostname = "example.com"
```

Blocks of type `module` are used to declare one or more modules, specifying
the _namespace_, _name_ and _provider_ for each:

```hcl
module "namespace" "name" "provider" {
  git_dir = "/var/lib/terraform-modules/namespace-name-provider"
}
```

Finally, blocks of type either `http` or `fastcgi` are used to declare one or
more listeners. The content of each of these blocks has the same structure,
and the type just decides which protocol is spoken on the resulting socket:

```hcl
http {
  address = "127.0.0.1:8081"

  # optional TLS configuration
  tls {
    cert_file = "/etc/terraform-registry/server.crt"
    key_file  = "/etc/terraform-registry/server.key"
  }
}
```

The server also supports systemd-style socket activation, by replacing the
`address` attribute with `socket_number` and specifying the index of the
socket to use from the set passed by the launching program.

## Module Git Repositories

The `git_dir` specified for a module is expected to be a _bare_ git repository
(that is, with no work tree) that contains one more more tags whose names
start with `v` and are followed by a valid module version string as defined
by Terraform.

When version numbers are requested, the list of tags is enumerated to determine
which versions are available, and the source code at the relevant tag is used
to produce a source archive when requested.

Git submodules are _not_ supported and will be ignored when producing a
module source archive.

## Service Discovery

As noted in [the main repository README](../../README.md), Terraform expects
to find a discovery document at the hostname given in the module source string.
For _this_ service, the document must contain a key named `modules.v1` whose
value is the base URL at which this server is deployed, with a trailing slash.

It is _not_ required for the server to run on the same hostname or port as
the discovery document itself.

## Authentication

This server does not _itself_ support authentication, since it's expected that
this will be provided by a frontend server that then accesses the modules
service via reverse-proxy or FastCGI.

Terraform requires that registry authentication be bearer-token-based, and so
use of JSON Web Tokens (JWT) or similar is recommended.

Terraform CLI does _not_ send credentials when it requests the source archive
for a module. Therefore it's necessary to make an exception in the authentication
wrapper for paths of the following form:

```
NAMESPACE/NAME/PROVIDER/VERSION/download/SHA1-HASH
```

The `SHA1-HASH` portion here is used as a basic protection mechanism to make
the URL hard to guess. However, it is not a time-limited signature and so
once exposed it will work indefinitely for the given module version. This
server is therefore not recommended for situations where modules themselves
contain secret information that must be properly protected.
