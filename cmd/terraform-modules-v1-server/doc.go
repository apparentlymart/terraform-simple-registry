// terraform-modules-v1-server provides a server that implements the Terraform
// module registry protocol version 1.
//
// Although it can be used directly via its built-in HTTP server, it is
// recommended to bind this program's services to a local TCP port or unix
// socket and expose it via a frontend server such as nginx, so that this
// frontend server can provide additional capabilities such as authentication,
// possibly a web UI to the registry (not included), etc.
package main
