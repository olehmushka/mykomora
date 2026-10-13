variable "hcloud_token" {
  type        = string
  sensitive   = true
  description = "Hetzner Cloud API token (project > Security > API tokens, read & write)."
}

variable "cloudflare_api_token" {
  type        = string
  sensitive   = true
  description = "Cloudflare API token allowed to edit R2 and create API tokens (Account > Workers R2 Storage: Edit, API Tokens: Edit)."
}

variable "cloudflare_account_id" {
  type        = string
  description = "Cloudflare account ID (R2 overview page)."
}

variable "domain" {
  type        = string
  description = "Hostname users open, e.g. yourname.duckdns.org. Used for bucket CORS."
}

variable "admin_ssh_public_key" {
  type        = string
  description = "Your public SSH key. Gets you onto the server as the 'admin' user."
}

variable "deploy_ssh_public_key" {
  type        = string
  description = "Public half of the key CI uses to deploy (private half is the DEPLOY_SSH_KEY GitHub secret). Gets the restricted 'deploy' user."
}

variable "ssh_allowed_cidrs" {
  type        = list(string)
  default     = ["0.0.0.0/0", "::/0"]
  description = "Who may reach port 22. GitHub-hosted runners have no fixed addresses, so this stays open; password login is disabled and only keys work."
}

variable "server_type" {
  type        = string
  default     = "cx23"
  description = "Hetzner plan; pick the cheapest shared-vCPU type with 2 GB RAM or more (`hcloud server-type list`). Names change over time."
}

variable "location" {
  type        = string
  default     = "hel1"
  description = "Hetzner location. hel1 (Helsinki) is among the closest to Ukraine."
}

variable "r2_location" {
  type        = string
  default     = "weur"
  description = "R2 location hint: wnam, enam, weur, eeur, apac."
}

variable "media_bucket" {
  type    = string
  default = "mykomora-media"
}

variable "backup_bucket" {
  type    = string
  default = "mykomora-backups"
}

variable "backup_retention_days" {
  type    = number
  default = 14
}
