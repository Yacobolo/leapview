mock_provider "hcloud" {}

variables {
  hcloud_token            = "test-token"
  ssh_allowed_cidrs       = ["203.0.113.10/32"]
  operator_ssh_public_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIE5D5x6Y3QxQ8mTvBz5K9eR3nM7pL1wC4sV6uA2f test"
}

run "secure_stateless_origin_plan" {
  command = plan

  assert {
    condition     = hcloud_primary_ip.site.auto_delete == false && hcloud_primary_ip.site.delete_protection
    error_message = "the reserved IPv4 must have independent deletion protection"
  }

  assert {
    condition = (
      hcloud_server.site.backups == false &&
      hcloud_server.site.delete_protection &&
      hcloud_server.site.rebuild_protection &&
      hcloud_server.site.shutdown_before_deletion
    )
    error_message = "the reproducible stateless server must have deletion and rebuild protection without stateful backups"
  }

  assert {
    condition     = length(hcloud_firewall.site.rule) == 3
    error_message = "the firewall must expose restricted SSH plus HTTP and HTTPS only"
  }

  assert {
    condition     = output.canonical_hostname == "leapview.dev"
    error_message = "the canonical hostname must remain explicit"
  }
}

run "reject_world_open_ipv4_ssh" {
  command = plan

  variables {
    ssh_allowed_cidrs = ["0.0.0.0/0"]
  }

  expect_failures = [var.ssh_allowed_cidrs]
}

run "reject_world_open_ipv6_ssh" {
  command = plan

  variables {
    ssh_allowed_cidrs = ["::/0"]
  }

  expect_failures = [var.ssh_allowed_cidrs]
}

run "reject_mutable_site_image" {
  command = plan

  variables {
    bootstrap_site_image = "ghcr.io/flidai/leapview-site:latest"
  }

  expect_failures = [var.bootstrap_site_image]
}

run "reject_product_image" {
  command = plan

  variables {
    bootstrap_site_image = "ghcr.io/flidai/leapview@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  }

  expect_failures = [var.bootstrap_site_image]
}

run "reject_mutable_caddy_image" {
  command = plan

  variables {
    caddy_image = "caddy:2"
  }

  expect_failures = [var.caddy_image]
}
