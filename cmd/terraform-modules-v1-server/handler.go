package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	version "github.com/hashicorp/go-version"

	"github.com/apparentlymart/terraform-simple-registry/config"
	"github.com/apparentlymart/terraform-simple-registry/module"
)

func makeHandler(modules config.Modules) http.Handler {
	ret := mux.NewRouter()

	ret.HandleFunc("/{namespace}/{name}", func(wr http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		namespace := vars["namespace"]
		name := vars["name"]

		byNamespace := modules[namespace]
		if byNamespace == nil {
			wr.WriteHeader(404)
			return
		}

		byName := byNamespace[name]
		if byName == nil {
			wr.WriteHeader(404)
			return
		}

		modules := make([]apiModule, 0)
		for provider, cfg := range byName {
			mod := module.Load(cfg.GitDir)
			if mod == nil {
				log.Printf("failed to open git repository at %s for module configured at %s", cfg.GitDir, cfg.DeclRange)
				continue
			}

			latest, err := mod.LatestVersion()
			if err != nil {
				log.Printf("failed to get latest version for %s: %s", cfg.DeclRange, err)
				continue
			}
			if latest == nil {
				continue
			}

			modules = append(modules, apiModule{
				ID:        fmt.Sprintf("%s/%s/%s/%s", namespace, name, provider, latest),
				Namespace: namespace,
				Name:      name,
				Provider:  provider,
				Version:   latest.String(),
			})
		}

		ret := apiModuleListResponse{
			Modules: modules,
		}
		buf, err := json.MarshalIndent(ret, "", "  ")
		if err != nil {
			wr.WriteHeader(500)
			log.Printf("error in JSON encoding: %s", err)
			return
		}
		wr.Write(buf)
	})

	ret.HandleFunc("/{namespace}/{name}/{provider}", func(wr http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		namespace := vars["namespace"]
		name := vars["name"]
		provider := vars["provider"]

		byNamespace := modules[namespace]
		if byNamespace == nil {
			wr.WriteHeader(404)
			return
		}

		byName := byNamespace[name]
		if byName == nil {
			wr.WriteHeader(404)
			return
		}

		cfg := byName[provider]

		if cfg == nil {
			wr.WriteHeader(404)
			return
		}

		mod := module.Load(cfg.GitDir)
		if mod == nil {
			log.Printf("failed to open git repository at %s for module configured at %s", cfg.GitDir, cfg.DeclRange)
			wr.WriteHeader(500)
			return
		}

		latest, err := mod.LatestVersion()
		if err != nil {
			log.Printf("failed to get latest version for %s: %s", cfg.DeclRange, err)
			wr.WriteHeader(500)
			return
		}
		if latest == nil {
			wr.WriteHeader(404)
			return
		}

		ret := &apiModule{
			ID:        fmt.Sprintf("%s/%s/%s/%s", namespace, name, provider, latest),
			Namespace: namespace,
			Name:      name,
			Provider:  provider,
			Version:   latest.String(),
		}
		buf, err := json.MarshalIndent(ret, "", "  ")
		if err != nil {
			wr.WriteHeader(500)
			log.Printf("error in JSON encoding: %s", err)
			return
		}
		wr.Write(buf)
	})

	ret.HandleFunc("/{namespace}/{name}/{provider}/{version}/download", func(wr http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		namespace := vars["namespace"]
		name := vars["name"]
		provider := vars["provider"]
		versionStr := vars["version"]

		byNamespace := modules[namespace]
		if byNamespace == nil {
			wr.WriteHeader(404)
			return
		}

		byName := byNamespace[name]
		if byName == nil {
			wr.WriteHeader(404)
			return
		}

		cfg := byName[provider]

		if cfg == nil {
			wr.WriteHeader(404)
			return
		}

		v, err := version.NewVersion(versionStr)
		if err != nil {
			wr.WriteHeader(404)
			return
		}

		mod := module.Load(cfg.GitDir)
		if mod == nil {
			log.Printf("failed to open git repository at %s for module configured at %s", cfg.GitDir, cfg.DeclRange)
			wr.WriteHeader(500)
			return
		}

		exists, err := mod.HasVersion(v)
		if err != nil {
			log.Printf("failed to check version %s for %s: %s", v, cfg.DeclRange, err)
			wr.WriteHeader(500)
			return
		}

		if !exists {
			wr.WriteHeader(404)
			return
		}

		wr.Header().Set("Content-Type", "application/x-tar")
		wr.WriteHeader(200)
		mod.WriteVersionTar(v, wr)
	})

	ret.HandleFunc("/{namespace}/{name}/{provider}/{version}", func(wr http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		namespace := vars["namespace"]
		name := vars["name"]
		provider := vars["provider"]
		versionStr := vars["version"]

		byNamespace := modules[namespace]
		if byNamespace == nil {
			wr.WriteHeader(404)
			return
		}

		byName := byNamespace[name]
		if byName == nil {
			wr.WriteHeader(404)
			return
		}

		cfg := byName[provider]

		if cfg == nil {
			wr.WriteHeader(404)
			return
		}

		v, err := version.NewVersion(versionStr)
		if err != nil {
			wr.WriteHeader(404)
			return
		}

		mod := module.Load(cfg.GitDir)
		if mod == nil {
			log.Printf("failed to open git repository at %s for module configured at %s", cfg.GitDir, cfg.DeclRange)
			wr.WriteHeader(500)
			return
		}

		exists, err := mod.HasVersion(v)
		if err != nil {
			log.Printf("failed to check version %s for %s: %s", v, cfg.DeclRange, err)
			wr.WriteHeader(500)
			return
		}

		if !exists {
			wr.WriteHeader(404)
			return
		}

		ret := &apiModule{
			ID:        fmt.Sprintf("%s/%s/%s/%s", namespace, name, provider, v.String()),
			Namespace: namespace,
			Name:      name,
			Provider:  provider,
			Version:   v.String(),
		}
		buf, err := json.MarshalIndent(ret, "", "  ")
		if err != nil {
			wr.WriteHeader(500)
			log.Printf("error in JSON encoding: %s", err)
			return
		}
		wr.Write(buf)
	})

	return ret
}

type apiModuleListResponse struct {
	Modules []apiModule `json:"modules"`
	Meta    *apiMeta    `json:"meta,omitempty"`
}

type apiMeta struct {
	Limit         string `json:"limit"`
	CurrentOffset string `json:"current_offset"`
	NextOffset    string `json:"next_offset"`
	PrevOffset    string `json:"prev_offset"`
}

type apiModule struct {
	ID        string `json:"id"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	Version   string `json:"version"`
}
