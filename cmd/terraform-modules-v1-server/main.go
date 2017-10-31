package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/apparentlymart/terraform-simple-registry/config"
	"github.com/hashicorp/hcl2/hcl"
	"github.com/hashicorp/hcl2/hclparse"
)

func realMain(args []string) int {
	parser := hclparse.NewParser()
	diagW := newDiagWriter(parser.Files())

	var diags hcl.Diagnostics

	if len(args) == 0 {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "No configuration files specified",
			Detail:   "At least one configuration file or configuration directory must be passed on the command line.",
		})
	}

	// Command line arguments are paths to either individual config files
	// or to directories containing config files.
	bodies := make([]hcl.Body, 0, len(args))
	for _, path := range args {
		info, err := os.Stat(path)
		if err != nil {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Configuration file not found",
				Detail:   fmt.Sprintf("Failed to read %s as a configuration file: %s", path, err),
			})
			continue
		}

		if info.IsDir() {
			entries, err := ioutil.ReadDir(path)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				path := filepath.Join(path, entry.Name())

				var file *hcl.File
				var bodyDiags hcl.Diagnostics
				if match, _ := filepath.Match("*.json", path); match {
					file, bodyDiags = parser.ParseJSONFile(path)
				} else {
					file, bodyDiags = parser.ParseHCLFile(path)
				}
				bodies = append(bodies, file.Body)
				diags = append(diags, bodyDiags...)
			}
		} else {
			var file *hcl.File
			var bodyDiags hcl.Diagnostics
			if match, _ := filepath.Match("*.json", path); match {
				file, bodyDiags = parser.ParseJSONFile(path)
			} else {
				file, bodyDiags = parser.ParseHCLFile(path)
			}
			bodies = append(bodies, file.Body)
			diags = append(diags, bodyDiags...)
		}
	}

	// Abort early if we had parse errors, since that means the bodies we loaded
	// are probably incomplete and may produce further errors on decoding.
	if diags.HasErrors() {
		diagW.WriteDiagnostics(diags)
		return 1
	}

	var body hcl.Body
	if len(bodies) == 1 {
		body = bodies[0]
	} else {
		body = hcl.MergeBodies(bodies)
	}

	cfg, cfgDiags := config.LoadModulesConfig(body)
	diags = append(diags, cfgDiags...)

	diagW.WriteDiagnostics(diags)
	if diags.HasErrors() {
		return 1
	}

	handler := makeHandler(cfg.Hostname, cfg.Modules)
	cfg.Listeners.ListenAndServe(handler) // does not return

	return 0
}

func newDiagWriter(files map[string]*hcl.File) hcl.DiagnosticWriter {
	if !terminal.IsTerminal(2) {
		return hcl.NewDiagnosticTextWriter(os.Stderr, files, 80, false)
	}

	wid, _, err := terminal.GetSize(2)
	if err != nil {
		wid = 80
	}

	return hcl.NewDiagnosticTextWriter(os.Stderr, files, uint(wid), true)
}

func main() {
	flag.Parse()
	args := flag.Args()

	status := realMain(args)
	os.Exit(status)
}
