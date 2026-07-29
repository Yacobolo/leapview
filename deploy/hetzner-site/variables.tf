variable "hcloud_token" {
  description = "Hetzner Cloud API token. Supply it through TF_VAR_hcloud_token."
  type        = string
  sensitive   = true
  nullable    = true
  default     = null
}

variable "name" {
  description = "Name prefix for permanent public-site resources."
  type        = string
  default     = "leapview-site"

  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$", var.name))
    error_message = "name must be a lowercase Hetzner-compatible resource name."
  }
}

variable "server_type" {
  description = "Small Hetzner server type for the stateless site origin."
  type        = string
  default     = "cx23"
}

variable "location" {
  description = "Explicit Hetzner location shared by the server and reserved IPv4."
  type        = string
  default     = "fsn1"
}

variable "server_image" {
  description = "Hetzner operating-system image used only when the server is created."
  type        = string
  default     = "ubuntu-24.04"
}

variable "ssh_allowed_cidrs" {
  description = "Restricted operator CIDRs allowed to reach SSH."
  type        = list(string)

  validation {
    condition = length(var.ssh_allowed_cidrs) > 0 && alltrue([
      for cidr in var.ssh_allowed_cidrs :
      can(cidrhost(cidr, 0)) && cidr != "0.0.0.0/0" && cidr != "::/0"
    ])
    error_message = "ssh_allowed_cidrs must contain valid restricted CIDRs; world-open SSH is forbidden."
  }
}

variable "operator_ssh_public_key" {
  description = "Public SSH key installed at initial server creation."
  type        = string
  sensitive   = true

  validation {
    condition = can(regex(
      "^(ssh-ed25519|ssh-rsa|ecdsa-sha2-nistp(256|384|521)) [A-Za-z0-9+/=]+(?: .*)?$",
      trimspace(var.operator_ssh_public_key),
    ))
    error_message = "operator_ssh_public_key must be a supported OpenSSH public key."
  }
}

variable "bootstrap_site_image" {
  description = "Initial public-site image. Routine digest changes are deployed by the release workflow, not Terraform."
  type        = string
  default     = "ghcr.io/yacobolo/leapview-site@sha256:1f4c7ac7fbaa332f96a907ff7daec10d49c26b3bbe5d3a16a7d48c274c5f168a"

  validation {
    condition = (
      can(regex(
        "^ghcr\\.io/flidai/leapview-site@sha256:[0-9a-f]{64}$",
        var.bootstrap_site_image,
      )) ||
      var.bootstrap_site_image == "ghcr.io/yacobolo/leapview-site@sha256:1f4c7ac7fbaa332f96a907ff7daec10d49c26b3bbe5d3a16a7d48c274c5f168a"
    )
    error_message = "bootstrap_site_image must be the canonical public-site image pinned by sha256 digest."
  }
}

variable "caddy_image" {
  description = "Caddy image pinned by immutable sha256 digest."
  type        = string
  default     = "caddy:2.10.2-alpine@sha256:4c6e91c6ed0e2fa03efd5b44747b625fec79bc9cd06ac5235a779726618e530d"

  validation {
    condition = can(regex(
      "^caddy:[A-Za-z0-9._-]+@sha256:[0-9a-f]{64}$",
      var.caddy_image,
    ))
    error_message = "caddy_image must be a Caddy image pinned by sha256 digest."
  }
}
