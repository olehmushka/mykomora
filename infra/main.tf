provider "hcloud" {
  token = var.hcloud_token
}

resource "hcloud_ssh_key" "admin" {
  name       = "mykomora-admin"
  public_key = var.admin_ssh_public_key
}

# A primary IP outlives the server. If the server is ever rebuilt, the address
# (and so the DuckDNS record and Google OAuth setup) stays the same.
resource "hcloud_primary_ip" "ipv4" {
  name        = "mykomora-ipv4"
  type        = "ipv4"
  location    = var.location
  auto_delete = false
}

resource "hcloud_firewall" "web" {
  name = "mykomora-web"

  rule {
    description = "SSH (key auth only)"
    direction   = "in"
    protocol    = "tcp"
    port        = "22"
    source_ips  = var.ssh_allowed_cidrs
  }

  rule {
    description = "HTTP (ACME challenge and redirect to HTTPS)"
    direction   = "in"
    protocol    = "tcp"
    port        = "80"
    source_ips  = ["0.0.0.0/0", "::/0"]
  }

  rule {
    description = "HTTPS"
    direction   = "in"
    protocol    = "tcp"
    port        = "443"
    source_ips  = ["0.0.0.0/0", "::/0"]
  }

  rule {
    description = "HTTP/3"
    direction   = "in"
    protocol    = "udp"
    port        = "443"
    source_ips  = ["0.0.0.0/0", "::/0"]
  }

  rule {
    description = "Ping"
    direction   = "in"
    protocol    = "icmp"
    source_ips  = ["0.0.0.0/0", "::/0"]
  }
}

resource "hcloud_server" "app" {
  name         = "mykomora"
  server_type  = var.server_type
  location     = var.location
  image        = "ubuntu-24.04"
  ssh_keys     = [hcloud_ssh_key.admin.id]
  firewall_ids = [hcloud_firewall.web.id]
  backups      = false

  public_net {
    ipv4_enabled = true
    ipv4         = hcloud_primary_ip.ipv4.id
    ipv6_enabled = true
  }

  # No secrets here: user_data is readable from inside the server and stored in
  # state. .env.prod is placed by hand (docs/runbook.md).
  user_data = templatefile("${path.module}/cloud-init.yaml.tftpl", {
    admin_ssh_public_key  = var.admin_ssh_public_key
    deploy_ssh_public_key = var.deploy_ssh_public_key
  })

  # Changing cloud-init later would otherwise force a rebuild of a server that
  # holds the database volume.
  lifecycle {
    ignore_changes = [user_data, ssh_keys, image]
  }
}
