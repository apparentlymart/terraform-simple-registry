package config

import (
	"fmt"

	"github.com/hashicorp/hcl2/gohcl"
	"github.com/hashicorp/hcl2/hcl"

	"github.com/hashicorp/terraform/svchost"
)

func loadHostnameConfig(body hcl.Body) (svchost.Hostname, hcl.Body, hcl.Diagnostics) {
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name:     "hostname",
				Required: true,
			},
		},
	}
	content, remain, diags := body.PartialContent(schema)
	if diags.HasErrors() {
		return svchost.Hostname(""), remain, diags
	}

	expr := content.Attributes["hostname"].Expr
	var raw string
	valDiags := gohcl.DecodeExpression(expr, nil, &raw)
	diags = append(diags, valDiags...)
	if valDiags.HasErrors() {
		return svchost.Hostname(""), remain, diags
	}

	ret, err := svchost.ForComparison(raw)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid hostname",
			Detail:   fmt.Sprintf("The given hostname is invalid: %s", err),
			Subject:  expr.Range().Ptr(),
		})
		return svchost.Hostname(""), remain, diags
	}

	return ret, remain, diags
}
