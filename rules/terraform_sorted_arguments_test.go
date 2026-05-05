package rules

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/terraform-linters/tflint-plugin-sdk/helper"
)

func TestTerraformSortedArguments(t *testing.T) {
	rule := NewTerraformSortedArgumentsRule()

	cases := []struct {
		name   string
		config string
		issues helper.Issues
		fixed  string
	}{
		// ── Alphabetical ordering (simple scalars) ──────────────────────────────
		{
			name: "simple attrs sorted - no issues",
			config: `
resource "aws_instance" "web" {
  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"
}
`,
			issues: helper.Issues{},
		},
		{
			name: "simple attrs unsorted - reports issue",
			config: `
resource "aws_instance" "web" {
  instance_type = "t2.micro"
  ami           = "ami-a1b2c3d4"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "ami" is not sorted: it should come before "instance_type"`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 6},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"
}
`,
		},

		// ── Category ordering: primitive before complex ──────────────────────────
		{
			name: "dict after simple with blank line - no issues",
			config: `
resource "aws_instance" "web" {
  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"

  tags = { Name = "web" }
}
`,
			issues: helper.Issues{},
		},
		{
			name: "dict before simple - category violation",
			config: `
resource "aws_instance" "web" {
  tags          = { Name = "web" }
  instance_type = "t2.micro"
}
`,
			issues: helper.Issues{
				{
					Rule: rule,
					Message: `argument "instance_type" (primitive variable) should come before "tags" (complex variable (list/map)): ` +
						orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 16},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  instance_type = "t2.micro"

  tags = { Name = "web" }
}
`,
		},
		{
			name: "for expression before simple - category violation",
			config: `
resource "aws_instance" "web" {
  member = [for u in var.users : u.id]
  name   = "web"
}
`,
			issues: helper.Issues{
				{
					Rule: rule,
					Message: `argument "name" (primitive variable) should come before "member" (complex variable (list/map)): ` +
						orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 7},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  name = "web"

  member = [for u in var.users : u.id]
}
`,
		},
		{
			name: "jsonencode before simple - category violation",
			config: `
resource "aws_instance" "web" {
  field_mappings = jsonencode({ key = "value" })
  name           = "web"
}
`,
			issues: helper.Issues{
				{
					Rule: rule,
					Message: `argument "name" (primitive variable) should come before "field_mappings" (complex variable (list/map)): ` +
						orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 7},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  name = "web"

  field_mappings = jsonencode({ key = "value" })
}
`,
		},
		{
			name: "array before simple - category violation",
			config: `
resource "aws_instance" "web" {
  security_groups = ["sg-1"]
  ami             = "ami-a1b2c3d4"
}
`,
			issues: helper.Issues{
				{
					Rule: rule,
					Message: `argument "ami" (primitive variable) should come before "security_groups" (complex variable (list/map)): ` +
						orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 6},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  security_groups = ["sg-1"]
}
`,
		},

		// ── Blank line before complex/block items ───────────────────────────────
		{
			name: "dict without blank line - blank line violation",
			config: `
resource "aws_instance" "web" {
  ami  = "ami-a1b2c3d4"
  tags = { Name = "web" }
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "tags" (complex variable (list/map))`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 7},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  tags = { Name = "web" }
}
`,
		},
		{
			name: "array without blank line - blank line violation",
			config: `
resource "aws_instance" "web" {
  ami             = "ami-a1b2c3d4"
  security_groups = ["sg-1"]
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "security_groups" (complex variable (list/map))`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 18},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  security_groups = ["sg-1"]
}
`,
		},
		{
			name: "nested block without blank line - blank line violation",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"
  root_block_device {
    volume_size = 20
  }
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "root_block_device" (nested block)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 20},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  root_block_device {
    volume_size = 20
  }
}
`,
		},
		{
			name: "nested block with blank line - no issues",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  root_block_device {
    volume_size = 20
  }
}
`,
			issues: helper.Issues{},
		},

		// ── Blank line between consecutive complex variables ────────────────────
		{
			name: "two maps without blank line - blank line violation",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  labels = { env = "prod" }
  tags   = { Name = "web" }
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "tags" (complex variable (list/map))`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 6, Column: 3},
						End:      hcl.Pos{Line: 6, Column: 7},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  labels = { env = "prod" }

  tags = { Name = "web" }
}
`,
		},
		{
			name: "two lists without blank line - blank line violation",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  cidr_blocks     = ["10.0.0.0/16"]
  security_groups = ["sg-1"]
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "security_groups" (complex variable (list/map))`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 6, Column: 3},
						End:      hcl.Pos{Line: 6, Column: 18},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  cidr_blocks = ["10.0.0.0/16"]

  security_groups = ["sg-1"]
}
`,
		},

		// ── Alphabetical within complex category ────────────────────────────────
		{
			name: "two dicts unsorted - sort violation",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  tags      = { Name = "web" }

  metadata  = { key = "val" }
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "metadata" is not sorted: it should come before "tags"`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 7, Column: 3},
						End:      hcl.Pos{Line: 7, Column: 11},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  metadata = { key = "val" }

  tags = { Name = "web" }
}
`,
		},

		// ── Nested block body is also checked ───────────────────────────────────
		{
			name: "nested block args unsorted - reports issue inside nested block",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  root_block_device {
    volume_size           = 20
    delete_on_termination = true
  }
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "delete_on_termination" is not sorted: it should come before "volume_size"`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 7, Column: 5},
						End:      hcl.Pos{Line: 7, Column: 26},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  root_block_device {
    delete_on_termination = true
    volume_size           = 20
  }
}
`,
		},

		// ── Meta-argument: source for modules ───────────────────────────────────
		{
			name: "module source first with blank line - no issues",
			config: `
module "database" {
  source = "../modules/database"

  db_size = 10
  region  = "us-east-1"
}
`,
			issues: helper.Issues{},
		},
		{
			name: "module source not first - category violation",
			config: `
module "database" {
  db_size = 10
  source  = "../modules/database"
}
`,
			issues: helper.Issues{
				{
					Rule: rule,
					Message: `argument "source" (source) should come before "db_size" (primitive variable): ` +
						orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 9},
					},
				},
			},
			fixed: `
module "database" {
  source = "../modules/database"

  db_size = 10
}
`,
		},
		{
			name: "module source first without blank line - blank line violation",
			config: `
module "database" {
  source  = "../modules/database"
  db_size = 10
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "db_size" (primitive variable)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 10},
					},
				},
			},
			fixed: `
module "database" {
  source = "../modules/database"

  db_size = 10
}
`,
		},

		// ── Meta-argument: for_each / count for resources ────────────────────────
		{
			name: "resource for_each first with blank line - no issues",
			config: `
resource "aws_instance" "web" {
  for_each = toset(["a", "b"])

  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"
}
`,
			issues: helper.Issues{},
		},
		{
			name: "resource for_each not first - category violation",
			config: `
resource "aws_instance" "web" {
  ami      = "ami-a1b2c3d4"
  for_each = toset(["a", "b"])
}
`,
			issues: helper.Issues{
				{
					Rule: rule,
					Message: `argument "for_each" (instantiation meta-argument (count/for_each)) should come before "ami" (primitive variable): ` +
						orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 11},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  for_each = toset(["a", "b"])

  ami = "ami-a1b2c3d4"
}
`,
		},
		{
			name: "resource for_each first without blank line - blank line violation",
			config: `
resource "aws_instance" "web" {
  for_each      = toset(["a", "b"])
  instance_type = "t2.micro"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "instance_type" (primitive variable)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 16},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  for_each = toset(["a", "b"])

  instance_type = "t2.micro"
}
`,
		},
		{
			name: "data source count first with blank line - no issues",
			config: `
data "aws_ami" "latest" {
  count = 1

  most_recent = true
}
`,
			issues: helper.Issues{},
		},

		// ── Category 0: provider argument ───────────────────────────────────────
		{
			name: "provider argument first - no issues",
			config: `
resource "aws_instance" "web" {
  provider = aws.us_east

  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"
}
`,
			issues: helper.Issues{},
		},
		{
			name: "provider argument after primitive - category violation",
			config: `
resource "aws_instance" "web" {
  ami      = "ami-a1b2c3d4"
  provider = aws.us_east
}
`,
			issues: helper.Issues{
				{
					Rule: rule,
					Message: `argument "provider" (provider) should come before "ami" (primitive variable): ` +
						orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 11},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  provider = aws.us_east

  ami = "ami-a1b2c3d4"
}
`,
		},
		{
			name: "provider without blank line before primitives - blank line violation",
			config: `
resource "aws_instance" "web" {
  provider      = aws.us_east
  instance_type = "t2.micro"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "instance_type" (primitive variable)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 16},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  provider = aws.us_east

  instance_type = "t2.micro"
}
`,
		},

		// ── Category 7: lifecycle meta-arguments at bottom ───────────────────────
		{
			name: "lifecycle at bottom with blank line - no issues",
			config: `
resource "aws_instance" "web" {
  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"

  lifecycle {
    create_before_destroy = true
  }
}
`,
			issues: helper.Issues{},
		},
		{
			name: "depends_on at bottom with blank line - no issues",
			config: `
resource "aws_instance" "web" {
  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"

  depends_on = [aws_vpc.main]
}
`,
			issues: helper.Issues{},
		},
		{
			name: "lifecycle before primitives - category violation",
			config: `
resource "aws_instance" "web" {
  lifecycle {
    create_before_destroy = true
  }
  ami = "ami-a1b2c3d4"
}
`,
			issues: helper.Issues{
				{
					Rule: rule,
					Message: `argument "ami" (primitive variable) should come before "lifecycle" (lifecycle meta-argument): ` +
						orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 6, Column: 3},
						End:      hcl.Pos{Line: 6, Column: 6},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  lifecycle {
    create_before_destroy = true
  }
}
`,
		},
		{
			name: "lifecycle without blank line - blank line violation",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"
  lifecycle {
    create_before_destroy = true
  }
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "lifecycle" (lifecycle meta-argument)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 12},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  lifecycle {
    create_before_destroy = true
  }
}
`,
		},
		{
			name: "depends_on and lifecycle both at bottom - blank line between each",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  depends_on = [aws_vpc.main]

  lifecycle {
    create_before_destroy = true
  }
}
`,
			issues: helper.Issues{},
		},
		{
			name: "depends_on and lifecycle without blank line between - blank line violation",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  depends_on = [aws_vpc.main]
  lifecycle {
    create_before_destroy = true
  }
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "lifecycle" (lifecycle meta-argument)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 6, Column: 3},
						End:      hcl.Pos{Line: 6, Column: 12},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  depends_on = [aws_vpc.main]

  lifecycle {
    create_before_destroy = true
  }
}
`,
		},

		// ── Consecutive same-type blocks: grouped without blank line ─────────────
		{
			name: "consecutive same-type blocks without blank line - no issues",
			config: `
resource "aws_autoscaling_group" "web" {
  ami = "ami-a1b2c3d4"

  tag {
    key   = "Name"
    value = "web"
  }
  tag {
    key   = "Env"
    value = "prod"
  }
}
`,
			issues: helper.Issues{},
		},
		{
			name: "consecutive dynamic blocks without blank line - blank line violation",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  dynamic "allowed_to_push" {
    for_each = var.push_users
    content {
      user_id = allowed_to_push.value
    }
  }
  dynamic "allowed_to_merge" {
    for_each = var.merge_users
    content {
      user_id = allowed_to_merge.value
    }
  }
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "dynamic" (nested block)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 11, Column: 3},
						End:      hcl.Pos{Line: 11, Column: 10},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  dynamic "allowed_to_push" {
    for_each = var.push_users
    content {
      user_id = allowed_to_push.value
    }
  }

  dynamic "allowed_to_merge" {
    for_each = var.merge_users
    content {
      user_id = allowed_to_merge.value
    }
  }
}
`,
		},
		{
			name: "consecutive dynamic blocks with blank line - no issues",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  dynamic "allowed_to_push" {
    for_each = var.push_users
    content {
      user_id = allowed_to_push.value
    }
  }

  dynamic "allowed_to_merge" {
    for_each = var.merge_users
    content {
      user_id = allowed_to_merge.value
    }
  }
}
`,
			issues: helper.Issues{},
		},
		{
			name: "consecutive different-type blocks without blank line - blank line violation",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  ebs_block_device {
    device_name = "/dev/sdb"
  }
  root_block_device {
    volume_size = 20
  }
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "root_block_device" (nested block)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 8, Column: 3},
						End:      hcl.Pos{Line: 8, Column: 20},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  ebs_block_device {
    device_name = "/dev/sdb"
  }

  root_block_device {
    volume_size = 20
  }
}
`,
		},
		{
			name: "consecutive different-type blocks with blank line - no issues",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  ebs_block_device {
    device_name = "/dev/sdb"
  }

  root_block_device {
    volume_size = 20
  }
}
`,
			issues: helper.Issues{},
		},

		// ── Edge cases: empty / single-item blocks ──────────────────────────────
		{
			name: "empty block - no issues",
			config: `
resource "aws_instance" "web" {
}
`,
			issues: helper.Issues{},
		},
		{
			name: "single argument block - no issues",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"
}
`,
			issues: helper.Issues{},
		},

		// ── Multiple violations in one block ────────────────────────────────────
		{
			name: "multiple violations in same block - all reported",
			config: `
resource "aws_instance" "web" {
  tags          = { Name = "web" }
  instance_type = "t2.micro"
  ami           = "ami-a1b2c3d4"
}
`,
			issues: helper.Issues{
				{
					Rule: rule,
					Message: `argument "instance_type" (primitive variable) should come before "tags" (complex variable (list/map)): ` +
						orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 16},
					},
				},
				{
					Rule:    rule,
					Message: `argument "ami" is not sorted: it should come before "instance_type"`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 5, Column: 3},
						End:      hcl.Pos{Line: 5, Column: 6},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"

  tags = { Name = "web" }
}
`,
		},

		// ── Complex after blocks: reorder ───────────────────────────────────────
		{
			name: "complex attr after block - category violation with fix",
			config: `
resource "google_compute_firewall" "test" {
  direction = "EGRESS"
  name      = "test"

  allow {
    protocol = "tcp"
  }

  destination_ranges = ["10.0.0.0/8"]
}
`,
			issues: helper.Issues{
				{
					Rule: rule,
					Message: `argument "destination_ranges" (complex variable (list/map)) should come before "allow" (nested block): ` +
						orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 10, Column: 3},
						End:      hcl.Pos{Line: 10, Column: 21},
					},
				},
			},
			fixed: `
resource "google_compute_firewall" "test" {
  direction = "EGRESS"
  name      = "test"

  destination_ranges = ["10.0.0.0/8"]

  allow {
    protocol = "tcp"
  }
}
`,
		},

		{
			name: "multiple complex attrs after multiple blocks - fix",
			config: `
resource "google_compute_firewall" "test" {
  direction = "EGRESS"
  name      = "test"
  network   = var.network_id

  allow {
    ports    = [5432]
    protocol = "tcp"
  }

  log_config {
    metadata = "INCLUDE_ALL_METADATA"
  }

  destination_ranges = [var.cloudsql_ip]

  target_service_accounts = [google_service_account.db.email]
}
`,
			issues: helper.Issues{
				{
					Rule: rule,
					Message: `argument "destination_ranges" (complex variable (list/map)) should come before "log_config" (nested block): ` +
						orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 16, Column: 3},
						End:      hcl.Pos{Line: 16, Column: 21},
					},
				},
			},
			fixed: `
resource "google_compute_firewall" "test" {
  direction = "EGRESS"
  name      = "test"
  network   = var.network_id

  destination_ranges = [var.cloudsql_ip]

  target_service_accounts = [google_service_account.db.email]

  allow {
    ports    = [5432]
    protocol = "tcp"
  }

  log_config {
    metadata = "INCLUDE_ALL_METADATA"
  }
}
`,
		},

		// ── Comments preserved during sort ───────────────────────────────────────
		{
			name: "comments preserved when reordering",
			config: `
resource "aws_instance" "web" {
  # Hardcoded value due to missing data source
  instance_type = "t2.micro"
  ami           = "ami-a1b2c3d4"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "ami" is not sorted: it should come before "instance_type"`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 5, Column: 3},
						End:      hcl.Pos{Line: 5, Column: 6},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"
  # Hardcoded value due to missing data source
  instance_type = "t2.micro"
}
`,
		},

		// ── Multiple resource blocks in same file ────────────────────────────────
		{
			name: "two resource blocks both checked independently",
			config: `
resource "aws_instance" "web" {
  instance_type = "t2.micro"
  ami           = "ami-a1b2c3d4"
}

resource "aws_instance" "db" {
  instance_type = "t2.micro"
  ami           = "ami-a1b2c3d4"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "ami" is not sorted: it should come before "instance_type"`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 6},
					},
				},
				{
					Rule:    rule,
					Message: `argument "ami" is not sorted: it should come before "instance_type"`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 9, Column: 3},
						End:      hcl.Pos{Line: 9, Column: 6},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"
}

resource "aws_instance" "db" {
  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"
}
`,
		},

		// ── count + for_each alphabetical ordering ───────────────────────────────
		{
			name: "count before for_each - no issues (alphabetical within catInstantiation)",
			config: `
resource "aws_instance" "web" {
  count    = 2
  for_each = toset(["a"])

  ami = "ami-a1b2c3d4"
}
`,
			issues: helper.Issues{},
		},
		{
			name: "for_each before count - sort violation within catInstantiation",
			config: `
resource "aws_instance" "web" {
  for_each = toset(["a"])
  count    = 2
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "count" is not sorted: it should come before "for_each"`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 8},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  count    = 2
  for_each = toset(["a"])
}
`,
		},

		// ── provider + count blank line ──────────────────────────────────────────
		{
			name: "provider then count with blank line - unwanted blank line",
			config: `
resource "aws_instance" "web" {
  provider = aws.us_east

  count = 2

  ami = "ami-a1b2c3d4"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `unexpected blank line before "count": provider, count/for_each, and source should form a single unit`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 5, Column: 3},
						End:      hcl.Pos{Line: 5, Column: 8},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  provider = aws.us_east
  count    = 2

  ami = "ami-a1b2c3d4"
}
`,
		},
		{
			name: "provider then count without blank line - no issues",
			config: `
resource "aws_instance" "web" {
  provider = aws.us_east
  count    = 2

  ami = "ami-a1b2c3d4"
}
`,
			issues: helper.Issues{},
		},

		// ── Three+ primitives: transitivity ─────────────────────────────────────
		{
			name: "three primitives sorted - no issues",
			config: `
resource "aws_instance" "web" {
  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"
  subnet_id     = "subnet-abc"
}
`,
			issues: helper.Issues{},
		},
		{
			name: "three primitives unsorted middle - sort violation",
			config: `
resource "aws_instance" "web" {
  ami           = "ami-a1b2c3d4"
  subnet_id     = "subnet-abc"
  instance_type = "t2.micro"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "instance_type" is not sorted: it should come before "subnet_id"`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 5, Column: 3},
						End:      hcl.Pos{Line: 5, Column: 16},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"
  subnet_id     = "subnet-abc"
}
`,
		},

		// ── Module: source + for_each blank line ─────────────────────────────────
		{
			name: "module for_each then source with blank line - unwanted blank line",
			config: `
module "database" {
  for_each = toset(["a", "b"])

  source = "../modules/database"

  db_size = 10
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `unexpected blank line before "source": provider, count/for_each, and source should form a single unit`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 5, Column: 3},
						End:      hcl.Pos{Line: 5, Column: 9},
					},
				},
			},
			fixed: `
module "database" {
  for_each = toset(["a", "b"])
  source   = "../modules/database"

  db_size = 10
}
`,
		},
		{
			name: "module for_each then source without blank line - no issues",
			config: `
module "database" {
  for_each = toset(["a", "b"])
  source   = "../modules/database"

  db_size = 10
}
`,
			issues: helper.Issues{},
		},

		// ── Multiple resource blocks with nested violations ─────────────────────
		{
			name: "multi-resource file with nested and outer violations",
			config: `
resource "google_compute_firewall" "fw1" {
  direction = "EGRESS"
  name      = "fw1"
  network   = var.network_id

  allow {
    ports    = [5432]
    protocol = "tcp"
  }

  destination_ranges = [var.cloudsql_ip]
}

resource "google_compute_firewall" "fw2" {
  direction = "EGRESS"
  name      = "fw2"
  network   = var.network_id

  allow {
    protocol = "tcp"
  }

  destination_ranges = [var.cloudsql_ip]
}
`,
			issues: helper.Issues{
				{
					Rule: rule,
					Message: `argument "destination_ranges" (complex variable (list/map)) should come before "allow" (nested block): ` +
						orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 12, Column: 3},
						End:      hcl.Pos{Line: 12, Column: 21},
					},
				},
				{
					Rule: rule,
					Message: `argument "destination_ranges" (complex variable (list/map)) should come before "allow" (nested block): ` +
						orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 24, Column: 3},
						End:      hcl.Pos{Line: 24, Column: 21},
					},
				},
			},
			fixed: `
resource "google_compute_firewall" "fw1" {
  direction = "EGRESS"
  name      = "fw1"
  network   = var.network_id

  destination_ranges = [var.cloudsql_ip]

  allow {
    ports    = [5432]
    protocol = "tcp"
  }
}

resource "google_compute_firewall" "fw2" {
  direction = "EGRESS"
  name      = "fw2"
  network   = var.network_id

  destination_ranges = [var.cloudsql_ip]

  allow {
    protocol = "tcp"
  }
}
`,
		},

		// ── Deeply nested blocks (3 levels) ─────────────────────────────────────
		{
			name: "three levels of nesting - inner violation reported",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  root_block_device {
    volume_size = 20

    nested_deep {
      z_attr = "z"
      a_attr = "a"
    }
  }
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "a_attr" is not sorted: it should come before "z_attr"`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 10, Column: 7},
						End:      hcl.Pos{Line: 10, Column: 13},
					},
				},
			},
			fixed: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  root_block_device {
    volume_size = 20

    nested_deep {
      a_attr = "a"
      z_attr = "z"
    }
  }
}
`,
		},
		{
			name: "inline comment preserved when reordering",
			config: `
resource "vault_generic_secret" "ci" {
  path = "secrets/ci"

  data_json = jsonencode({
    max_request_duration = "86400s" # 24 hours
  })

  disable_read = true
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "disable_read" (primitive variable) should come before "data_json" (complex variable (list/map)): ` + orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 9, Column: 3},
						End:      hcl.Pos{Line: 9, Column: 15},
					},
				},
			},
			fixed: `
resource "vault_generic_secret" "ci" {
  disable_read = true
  path         = "secrets/ci"

  data_json = jsonencode({
    max_request_duration = "86400s" # 24 hours
  })
}
`,
		},
		{
			name: "compact treated as complex type",
			config: `
resource "google_compute_instance" "web" {
  for_each = local.instances

  name = "instance"
  resource_policies = compact([
    lookup(each.value, "policy", false) ? google_compute_resource_policy.nightly.self_link : null,
  ])
  zone = local.zone
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "resource_policies" (complex variable (list/map))`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 6, Column: 3},
						End:      hcl.Pos{Line: 6, Column: 20},
					},
				},
				{
					Rule:    rule,
					Message: `argument "zone" (primitive variable) should come before "resource_policies" (complex variable (list/map)): ` + orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 9, Column: 3},
						End:      hcl.Pos{Line: 9, Column: 7},
					},
				},
			},
			fixed: `
resource "google_compute_instance" "web" {
  for_each = local.instances

  name = "instance"
  zone = local.zone

  resource_policies = compact([
    lookup(each.value, "policy", false) ? google_compute_resource_policy.nightly.self_link : null,
  ])
}
`,
		},
		{
			name: "merge treated as complex type",
			config: `
resource "google_compute_instance" "web" {
  for_each = local.instances

  labels = merge(
    { "app" = "web" },
    lookup(each.value, "labels", {})
  )
  name = "instance"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "name" (primitive variable) should come before "labels" (complex variable (list/map)): ` + orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 9, Column: 3},
						End:      hcl.Pos{Line: 9, Column: 7},
					},
				},
			},
			fixed: `
resource "google_compute_instance" "web" {
  for_each = local.instances

  name = "instance"

  labels = merge(
    { "app" = "web" },
    lookup(each.value, "labels", {})
  )
}
`,
		},
		{
			name: "concat treated as complex type",
			config: `
locals {
  roles = concat([
    "roles/viewer",
  ], var.extra_roles)
  project = "my-project"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "project" (primitive variable) should come before "roles" (complex variable (list/map)): ` + orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 6, Column: 3},
						End:      hcl.Pos{Line: 6, Column: 10},
					},
				},
			},
			fixed: `
locals {
  project = "my-project"

  roles = concat([
    "roles/viewer",
  ], var.extra_roles)
}
`,
		},
		{
			name: "block comment preserved when reordering",
			config: `
resource "example" "test" {
  name = "test"
  /**
    * This maps claims from the provider.
    */
  attribute_mapping = {
    "attr" = "value"
  }
  display_name = "hello"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "display_name" (primitive variable) should come before "attribute_mapping" (complex variable (list/map)): ` + orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 10, Column: 3},
						End:      hcl.Pos{Line: 10, Column: 15},
					},
				},
			},
			fixed: `
resource "example" "test" {
  display_name = "hello"
  name         = "test"

  /**
  * This maps claims from the provider.
  */
  attribute_mapping = {
    "attr" = "value"
  }
}
`,
		},
		{
			name: "toset treated as complex type",
			config: `
resource "example" "test" {
  members = toset([
    "user:alice@example.com",
    "user:bob@example.com",
  ])
  name = "test"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "name" (primitive variable) should come before "members" (complex variable (list/map)): ` + orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 7, Column: 3},
						End:      hcl.Pos{Line: 7, Column: 7},
					},
				},
			},
			fixed: `
resource "example" "test" {
  name = "test"

  members = toset([
    "user:alice@example.com",
    "user:bob@example.com",
  ])
}
`,
		},
		{
			name: "heredoc treated as complex type",
			config: `
module "alert" {
  source = "./modules/alert"

  display_name          = "my alert"
  documentation_content = <<-EOF
    - *Threshold*: 1
    - *Dashboard*: <https://example.com|Click here>
  EOF
  notification_channels = local.channels
  severity              = "WARNING"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "documentation_content" (complex variable (list/map))`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 6, Column: 3},
						End:      hcl.Pos{Line: 6, Column: 24},
					},
				},
				{
					Rule:    rule,
					Message: `argument "notification_channels" (primitive variable) should come before "documentation_content" (complex variable (list/map)): ` + orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 10, Column: 3},
						End:      hcl.Pos{Line: 10, Column: 24},
					},
				},
			},
			fixed: `
module "alert" {
  source = "./modules/alert"

  display_name          = "my alert"
  notification_channels = local.channels
  severity              = "WARNING"

  documentation_content = <<-EOF
    - *Threshold*: 1
    - *Dashboard*: <https://example.com|Click here>
  EOF
}
`,
		},
		{
			name: "templatefile treated as complex type",
			config: `
resource "example" "test" {
  name     = "test"
  metadata = templatefile("${path.module}/template.yaml", {
    project = var.project
  })
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "metadata" (complex variable (list/map))`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 11},
					},
				},
			},
			fixed: `
resource "example" "test" {
  name = "test"

  metadata = templatefile("${path.module}/template.yaml", {
    project = var.project
  })
}
`,
		},
		{
			name: "content attribute needs blank line before it",
			config: `
resource "google_storage_bucket_object" "test" {
  bucket = google_storage_bucket.test.name
  name   = "config.yaml"
  content = templatefile("${path.module}/config.yaml", {
    endpoint = var.endpoint
  })
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "content" (complex variable (list/map))`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 5, Column: 3},
						End:      hcl.Pos{Line: 5, Column: 10},
					},
				},
			},
			fixed: `
resource "google_storage_bucket_object" "test" {
  bucket = google_storage_bucket.test.name
  name   = "config.yaml"

  content = templatefile("${path.module}/config.yaml", {
    endpoint = var.endpoint
  })
}
`,
		},
		{
			name: "content block in dynamic does not need blank line",
			config: `
resource "example" "test" {
  dynamic "setting" {
    for_each = var.settings
    content {
      name  = setting.value.name
      value = setting.value.value
    }
  }
}
`,
			issues: helper.Issues{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := helper.TestRunner(t, map[string]string{"main.tf": tc.config})
			if err := rule.Check(runner); err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			helper.AssertIssues(t, tc.issues, runner.Issues)

			if len(tc.issues) > 0 && tc.fixed == "" {
				t.Fatal("test case reports issues but has no 'fixed' assertion")
			}
			if tc.fixed != "" {
				got := string(runner.Changes()["main.tf"])
				if got != tc.fixed {
					t.Errorf("fix mismatch:\nwant:\n%s\ngot:\n%s", tc.fixed, got)
				}
			}
		})
	}
}
