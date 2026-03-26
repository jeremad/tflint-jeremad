# tflint-ruleset-jeremad

A [TFLint](https://github.com/terraform-linters/tflint) ruleset plugin that enforces a canonical argument ordering style in Terraform configuration files.

## Rules

| Rule | Description | Severity |
|------|-------------|----------|
| [terraform_sorted_arguments](rules/terraform_sorted_arguments.go) | Enforces canonical argument ordering within blocks | Warning |

### `terraform_sorted_arguments`

Arguments inside a block must appear in the following order, top to bottom:

1. **`provider`** — the provider alias argument
2. **Instantiation meta-arguments** — `count`, `for_each`
3. **`source`** — module source (modules only)
4. **Primitive variables** — booleans, numbers, strings, references, function calls; sorted alphabetically within this group
5. **Complex variables** — lists (`[…]`) and maps (`{…}`); sorted alphabetically, each separated from the previous group by a blank line
6. **Nested blocks** — HCL sub-blocks; separated by a blank line, sorted alphabetically (consecutive blocks of the same type may be grouped without a blank line between them)
7. **Lifecycle meta-arguments** — `lifecycle`, `depends_on`; each preceded by a blank line

## Installation

### Via `tflint --init` (recommended)

Add the following to your `.tflint.hcl`:

```hcl
plugin "jeremad" {
  enabled = true
  version = "0.1.0"
  source  = "github.com/jeremad/tflint-jeremad"
}
```

Then run:

```sh
tflint --init
```

### Manual

Download the binary for your platform from the [releases page](https://github.com/jeremad/tflint-jeremad/releases), place it in `~/.tflint.d/plugins/`, and make it executable.

### From source

```sh
git clone https://github.com/jeremad/tflint-jeremad.git
cd tflint-jeremad
make install
```

## Usage

```sh
tflint --enable-plugin=jeremad
```

Or with the `.tflint.hcl` config above, simply run:

```sh
tflint
```

## Example

The following block will trigger violations:

```hcl
resource "aws_instance" "web" {
  tags          = { Name = "web" }   # complex variable before primitives
  instance_type = "t2.micro"
  ami           = "ami-a1b2c3d4"     # primitives not sorted alphabetically
  lifecycle {                        # no blank line before lifecycle block
    create_before_destroy = true
  }
}
```

Corrected version:

```hcl
resource "aws_instance" "web" {
  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"

  tags = { Name = "web" }

  lifecycle {
    create_before_destroy = true
  }
}
```

## Development

```sh
make test    # run tests
make build   # build the binary
make install # install to ~/.tflint.d/plugins
```

## Release

Releases are automated via GitHub Actions and [GoReleaser](https://goreleaser.com). Push a version tag to trigger a release:

```sh
git tag v0.1.1
git push origin v0.1.1
```

Binaries for all supported platforms will be built, signed, and published automatically.
