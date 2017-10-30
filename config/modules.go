package config

import (
	"fmt"

	"github.com/hashicorp/hcl2/gohcl"

	"github.com/hashicorp/hcl2/hcl"
)

// ModulesConfig is the root type of a configuration for a modules server.
type ModulesConfig struct {
	Listeners Listeners
	Modules   Modules
}

// LoadModulesConfig processes a raw HCL Body into a configuration for a
// module registry server.
//
// If the returned diagnostics has errors, the returned configuration may
// be incomplete or invalid. Otherwise, the returned configuration is complete
// and guaranteed to be statically valid. (References to files, TCP ports,
// etc are not checked until they are used.)
func LoadModulesConfig(body hcl.Body) (*ModulesConfig, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	listeners, remain, listenersDiags := loadListenersConfig(body)
	body = remain
	diags = append(diags, listenersDiags...)

	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "module",
				LabelNames: []string{"namespace", "name", "provider"},
			},
		},
	}
	content, modulesDiags := body.Content(schema)
	diags = append(diags, modulesDiags...)

	type module struct {
		GitDir string `hcl:"git_dir,attr"`
	}

	modules := make(Modules)
	for _, block := range content.Blocks {
		namespace, name, provider := block.Labels[0], block.Labels[1], block.Labels[2]
		declRange := hcl.RangeBetween(block.TypeRange, block.LabelRanges[2])
		if modules[namespace] == nil {
			modules[namespace] = make(map[string]map[string]*Module)
		}
		if modules[namespace][name] == nil {
			modules[namespace][name] = make(map[string]*Module)
		}
		if existing, exists := modules[namespace][name][provider]; exists {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Duplicate module declaration",
				Detail:   fmt.Sprintf("A module block for %q %q %q was already declared at %s.", namespace, name, provider, existing.DeclRange),
				Subject:  &declRange,
			})
			continue
		}

		var raw module
		bodyDiags := gohcl.DecodeBody(block.Body, nil, &raw)
		diags = append(diags, bodyDiags...)
		if bodyDiags.HasErrors() {
			continue
		}

		modules[namespace][name][provider] = &Module{
			GitDir:    raw.GitDir,
			DeclRange: declRange,
		}
	}

	return &ModulesConfig{
		Listeners: listeners,
		Modules:   modules,
	}, diags
}

// ModulesConfig is a map of many modules to serve from a module registry
// service. The keys of each respective map are the "namespace" (an arbitrary
// container that may be used to model internal departments, etc), the module
// name, and the provider.
type Modules map[string]map[string]map[string]*Module

// ModuleConfig is the configuration for a single module to be served from
// a module registry service.
type Module struct {
	GitDir    string
	DeclRange hcl.Range
}
