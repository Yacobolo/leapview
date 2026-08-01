terraform {
  required_version = ">= 1.13.0"

  # The remote backend keeps state in HCP Terraform while allowing
  # `terraform init -backend=false` to validate this module without personal
  # HCP credentials. The workspace is configured for local execution by the
  # explicit provisioning script.
  backend "remote" {}

  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.52"
    }
  }
}

provider "hcloud" {
  token = var.hcloud_token
}
