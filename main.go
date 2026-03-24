package main

import (
	"github.com/jeremad/tflint-ruleset-jeremad/rules"
	"github.com/terraform-linters/tflint-plugin-sdk/plugin"
	"github.com/terraform-linters/tflint-plugin-sdk/tflint"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		RuleSet: &tflint.BuiltinRuleSet{
			Name:    "jeremad",
			Version: "0.1.0",
			Rules: []tflint.Rule{
				rules.NewTerraformSortedArgumentsRule(),
			},
		},
	})
}
