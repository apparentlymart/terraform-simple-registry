package config

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/fcgi"
	"strings"

	"github.com/coreos/go-systemd/activation"
	"github.com/hashicorp/hcl2/gohcl"
	"github.com/hashicorp/hcl2/hcl"
)

type Listeners map[Listener]struct{}

// ListenAndServe attempts to listen on all of the listeners in the receiver
// and then serves requests with the given handler on those that are successful.
//
// Each listener operates in its own goroutine, which may in turn spawn
// additional goroutines as requests arrive.
//
// This function never returns. If any of the listeners fail to listen, errors
// will be logged using the "log" package.
func (ls Listeners) ListenAndServe(handler http.Handler) {
	for l := range ls {
		go func(l Listener) {
			err := l.ListenAndServe(handler)
			if err != nil {
				log.Printf("failed to listen: %s", err)
			}
		}(l)
	}

	// Block forever
	never := make(chan struct{})
	<-never
}

func loadListenersConfig(body hcl.Body) (Listeners, hcl.Body, hcl.Diagnostics) {
	// We use some local types here to make our decoding a bit more declarative,
	// and then produce the _real_ listener types before we return.

	type tls struct {
		CertFile string `hcl:"cert_file,attr"`
		KeyFile  string `hcl:"key_file,attr"`
	}
	type listener struct {
		Address      *string `hcl:"address,attr"`
		SocketNumber *int    `hcl:"socket_number,attr"`
		TLS          *tls    `hcl:"tls,block"`
	}
	type listenersConfig struct {
		HTTP    []listener `hcl:"http,block"`
		FastCGI []listener `hcl:"fastcgi,block"`
		Remain  hcl.Body   `hcl:",remain"`
	}

	var raw listenersConfig
	diags := gohcl.DecodeBody(body, nil, &raw)

	ret := make(map[Listener]struct{})

	listenerConf := func(lc listener) listenerConfig {
		if lc.Address != nil && lc.SocketNumber != nil {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid listener configuration",
				Detail:   "Cannot set both \"address\" and \"socket_number\" for the same listener.",
				// FIXME: We don't have access to the source range here :(
			})
		}

		var socket socketConfig

		switch {
		case lc.Address != nil:
			if strings.HasPrefix(*lc.Address, "/") {
				socket = unixSocketPath(*lc.Address)
			} else {
				socket = tcpAddress(*lc.Address)
			}
		case lc.SocketNumber != nil:
			socket = socketActivationIndex(*lc.SocketNumber)
		default:
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid listener configuration",
				Detail:   "A listener must have either \"address\" or \"socket_number\" set.",
				// FIXME: We don't have access to the source range here :(
			})
			socket = tcpAddress("") // placeholder value
		}

		var tls *listenerTLS
		if lc.TLS != nil {
			tls = &listenerTLS{
				CertFile: lc.TLS.CertFile,
				KeyFile:  lc.TLS.KeyFile,
			}
		}

		return listenerConfig{
			Socket: socket,
			TLS:    tls,
		}
	}

	for _, lc := range raw.HTTP {
		ret[httpListener{conf: listenerConf(lc)}] = struct{}{}
	}
	for _, lc := range raw.FastCGI {
		ret[fastCGIListener{conf: listenerConf(lc)}] = struct{}{}
	}

	return ret, raw.Remain, diags
}

type Listener interface {
	ListenAndServe(handler http.Handler) error
}

type httpListener struct {
	conf listenerConfig
}

func (l httpListener) ListenAndServe(handler http.Handler) error {
	socket, err := l.conf.Listen()
	if err != nil {
		return err
	}

	server := http.Server{
		Handler: handler,
	}
	return server.Serve(socket)
}

type fastCGIListener struct {
	conf listenerConfig
}

func (l fastCGIListener) ListenAndServe(handler http.Handler) error {
	socket, err := l.conf.Listen()
	if err != nil {
		return err
	}

	return fcgi.Serve(socket, handler)
}

type listenerConfig struct {
	Socket socketConfig
	TLS    *listenerTLS
}

func (lc *listenerConfig) Listen() (net.Listener, error) {
	l, err := lc.Socket.Listen()
	if err != nil {
		return nil, err
	}

	if lc.TLS != nil {
		cert, err := tls.LoadX509KeyPair(lc.TLS.CertFile, lc.TLS.KeyFile)
		if err != nil {
			return nil, err
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}

		l = tls.NewListener(l, tlsConfig)
	}

	return l, nil
}

type listenerTLS struct {
	CertFile string
	KeyFile  string
}

type socketConfig interface {
	Listen() (net.Listener, error)
}

type tcpAddress string

func (a tcpAddress) Listen() (net.Listener, error) {
	return net.Listen("tcp", string(a))
}

type unixSocketPath string

func (a unixSocketPath) Listen() (net.Listener, error) {
	return net.Listen("unix", string(a))
}

type socketActivationIndex int

func (i socketActivationIndex) Listen() (net.Listener, error) {
	listeners, err := activation.Listeners(false)
	if err != nil {
		return nil, err
	}
	idx := int(i)
	if idx >= len(listeners) {
		return nil, fmt.Errorf("insufficent sockets passed by supervisor: need at least %d but only got %d", idx+1, len(listeners))
	}
	ret := listeners[idx]
	if ret == nil {
		return nil, fmt.Errorf("supervisor-passed socket %d is not stream-based", idx)
	}
	return ret, nil
}
