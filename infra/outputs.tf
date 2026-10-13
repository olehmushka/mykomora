output "server_ipv4" {
  description = "Point the DuckDNS record at this address."
  value       = hcloud_primary_ip.ipv4.ip_address
}

output "ssh_admin" {
  value = "ssh admin@${hcloud_primary_ip.ipv4.ip_address}"
}
