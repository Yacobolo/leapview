locals {
  ssh_public_key_path  = pathexpand(var.ssh_public_key_path)
  create_ssh_key       = trimspace(var.ssh_public_key_path) != "" && fileexists(local.ssh_public_key_path)
  ssh_private_key_path = endswith(local.ssh_public_key_path, ".pub") ? trimsuffix(local.ssh_public_key_path, ".pub") : local.ssh_public_key_path
  ssh_identity_arg     = local.create_ssh_key ? " -i ${local.ssh_private_key_path}" : ""
  domain               = trimspace(var.domain) != "" ? trimspace(var.domain) : "${replace(hcloud_primary_ip.libredash.ip_address, ".", "-")}.sslip.io"
  labels = {
    app = "libredash"
  }
}

resource "hcloud_primary_ip" "libredash" {
  name        = "${var.name}-ipv4"
  location    = var.location
  type        = "ipv4"
  auto_delete = false
  labels      = local.labels
}

resource "hcloud_ssh_key" "local" {
  count      = local.create_ssh_key ? 1 : 0
  name       = "${var.name}-local"
  public_key = file(local.ssh_public_key_path)
  labels     = local.labels
}

resource "hcloud_firewall" "libredash" {
  name   = "${var.name}-firewall"
  labels = local.labels

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = var.ssh_allowed_cidrs
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "80"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "443"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
}

resource "hcloud_server" "libredash" {
  name        = var.name
  server_type = var.server_type
  image       = var.image
  location    = var.location
  ssh_keys    = concat(var.ssh_key_ids, hcloud_ssh_key.local[*].id)
  firewall_ids = [
    hcloud_firewall.libredash.id,
  ]
  labels = local.labels

  public_net {
    ipv4_enabled = true
    ipv4         = hcloud_primary_ip.libredash.id
    ipv6_enabled = true
  }

  user_data = templatefile("${path.module}/cloud-init.yaml.tftpl", {
    domain      = local.domain
    admin_email = var.admin_email
    repo_url    = var.repo_url
    repo_ref    = var.repo_ref
  })
}
