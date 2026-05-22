package rules

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/terraform-linters/tflint-plugin-sdk/tflint"
)

var forbiddenResources = map[string]string{
	"google_project_iam_policy": "google_project_iam_policy is authoritative and replaces the entire IAM policy on a project; use google_project_iam_binding or google_project_iam_member instead",
}

type TerraformForbiddenResourcesRule struct {
	baseRule
}

func NewTerraformForbiddenResourcesRule() *TerraformForbiddenResourcesRule {
	return &TerraformForbiddenResourcesRule{baseRule{name: "terraform_forbidden_resources"}}
}

func (r *TerraformForbiddenResourcesRule) Severity() tflint.Severity { return tflint.ERROR }

func (r *TerraformForbiddenResourcesRule) Check(runner tflint.Runner) error {
	return forEachFile(runner, func(file *hcl.File) error {
		return r.checkFile(runner, file)
	})
}

func (r *TerraformForbiddenResourcesRule) checkFile(runner tflint.Runner, file *hcl.File) error {
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil
	}
	for _, block := range body.Blocks {
		if block.Type != "resource" || len(block.Labels) < 1 {
			continue
		}
		reason, forbidden := forbiddenResources[block.Labels[0]]
		if !forbidden {
			continue
		}
		msg := fmt.Sprintf("resource %q is forbidden: %s", block.Labels[0], reason)
		if err := runner.EmitIssue(r, msg, block.LabelRanges[0]); err != nil {
			return err
		}
	}
	return nil
}
