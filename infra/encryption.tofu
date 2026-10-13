# OpenTofu-only (Terraform ignores *.tofu files). The state holds the R2 API
# token, so it is encrypted client-side before it reaches the bucket.
# The passphrase comes from the environment, never from a file:
#
#   export TF_VAR_state_passphrase='...16+ characters...'
variable "state_passphrase" {
  type      = string
  sensitive = true
}

terraform {
  encryption {
    key_provider "pbkdf2" "main" {
      passphrase = var.state_passphrase
    }
    method "aes_gcm" "main" {
      keys = key_provider.pbkdf2.main
    }
    state {
      method = method.aes_gcm.main
    }
    plan {
      method = method.aes_gcm.main
    }
  }
}
