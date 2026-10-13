terraform {
  required_version = ">= 1.8.0"

  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.49"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.0"
    }
  }

  # State lives in a small R2 bucket created once by hand (infra/README.md).
  # Everything account-specific comes from `tofu init -backend-config=backend.hcl`,
  # because a backend block cannot use variables.
  backend "s3" {
    key    = "mykomora/terraform.tfstate"
    region = "auto"

    # R2 speaks the S3 API but is not AWS.
    skip_credentials_validation = true
    skip_region_validation      = true
    skip_requesting_account_id  = true
    skip_metadata_api_check     = true
    skip_s3_checksum            = true
    use_path_style              = true
  }
}
