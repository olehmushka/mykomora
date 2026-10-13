resource "cloudflare_r2_bucket" "media" {
  account_id = var.cloudflare_account_id
  name       = var.media_bucket
  location   = var.r2_location
}

resource "cloudflare_r2_bucket" "backups" {
  account_id = var.cloudflare_account_id
  name       = var.backup_bucket
  location   = var.r2_location
}

# The browser uploads and downloads with presigned URLs, so the media bucket
# must answer cross-origin requests from the app's origin and nobody else's.
resource "cloudflare_r2_bucket_cors" "media" {
  account_id  = var.cloudflare_account_id
  bucket_name = cloudflare_r2_bucket.media.name

  rules = [{
    allowed = {
      origins = ["https://${var.domain}"]
      methods = ["GET", "PUT"]
      headers = ["*"]
    }
    expose_headers  = ["ETag"]
    max_age_seconds = 3600
  }]
}

# Retention for nightly dumps (deploy/backup.sh).
resource "cloudflare_r2_bucket_lifecycle" "backups" {
  account_id  = var.cloudflare_account_id
  bucket_name = cloudflare_r2_bucket.backups.name

  rules = [{
    id      = "expire-old-dumps"
    enabled = true
    conditions = {
      prefix = "postgres/"
    }
    delete_objects_transition = {
      condition = {
        type    = "Age"
        max_age = var.backup_retention_days * 86400
      }
    }
  }]
}

# S3 credentials for the app and the backup script, scoped to the two buckets.
# R2 derives S3 keys from an API token: access key = token id,
# secret = sha256(token value).
data "cloudflare_api_token_permission_groups_list" "r2" {
  name = "Workers%20R2%20Storage%20Bucket%20Item%20Write"
}

resource "cloudflare_api_token" "r2_app" {
  name = "mykomora-r2-app"

  policies = [{
    effect = "allow"
    permission_groups = [
      for g in data.cloudflare_api_token_permission_groups_list.r2.result : { id = g.id }
    ]
    resources = {
      "com.cloudflare.edge.r2.bucket.${var.cloudflare_account_id}_default_${cloudflare_r2_bucket.media.name}"   = "*"
      "com.cloudflare.edge.r2.bucket.${var.cloudflare_account_id}_default_${cloudflare_r2_bucket.backups.name}" = "*"
    }
  }]
}
