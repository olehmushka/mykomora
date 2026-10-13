output "server_ipv4" {
  description = "Point the DuckDNS record at this address."
  value       = hcloud_primary_ip.ipv4.ip_address
}

output "ssh_admin" {
  value = "ssh admin@${hcloud_primary_ip.ipv4.ip_address}"
}

output "s3_endpoint" {
  description = "S3_ENDPOINT for deploy/.env.prod."
  value       = "https://${var.cloudflare_account_id}.r2.cloudflarestorage.com"
}

output "s3_access_key_id" {
  description = "S3_ACCESS_KEY_ID for deploy/.env.prod."
  value       = cloudflare_api_token.r2_app.id
}

output "s3_secret_access_key" {
  description = "S3_SECRET_ACCESS_KEY for deploy/.env.prod (read with: tofu output -raw s3_secret_access_key)."
  value       = sha256(cloudflare_api_token.r2_app.value)
  sensitive   = true
}
