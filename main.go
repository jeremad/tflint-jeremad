package main

import (
	"github.com/terraform-linters/tflint-plugin-sdk/plugin"
	"github.com/terraform-linters/tflint-plugin-sdk/tflint"

	"github.com/jeremad/tflint-ruleset-jeremad/rules"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		RuleSet: &tflint.BuiltinRuleSet{
			Name:    "jeremad",
			Version: "0.2.0",
			Rules: []tflint.Rule{
				rules.NewTerraformSortedArgumentsRule(),
				rules.NewTerraformSortedVariablesRule(),
			},
		},
	})
}
