resource "hcloud_ssh_key" "operator" {
  name       = "leapview-rtest-poc"
  public_key = file(var.ssh_public_key_path)
}

resource "hcloud_firewall" "runner" {
  name = "leapview-rtest-poc"

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = var.ssh_allowed_cidrs
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "443"
    source_ips = var.service_allowed_cidrs
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "50052"
    source_ips = var.service_allowed_cidrs
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "1235"
    source_ips = var.service_allowed_cidrs
  }
}

resource "hcloud_server" "runner" {
  name         = "leapview-rtest-poc"
  image        = "ubuntu-24.04"
  server_type  = var.server_type
  location     = var.location
  ssh_keys     = [hcloud_ssh_key.operator.id]
  firewall_ids = [hcloud_firewall.runner.id]
  user_data    = file("${path.module}/cloud-init.yaml")

  public_net {
    ipv4_enabled = true
    ipv6_enabled = false
  }

  labels = {
    project     = "rtest"
    environment = "poc"
    managed_by  = "terraform"
  }
}
