variable "hcloud_token" {
  description = "Hetzner Cloud API token. Supplied through TF_VAR_hcloud_token."
  type        = string
  sensitive   = true
}

variable "server_type" {
  description = "Hetzner server type for the single-worker POC."
  type        = string
  default     = "cpx32"
}

variable "service_allowed_cidrs" {
  description = "CIDRs allowed to reach the HTTPS control plane and mTLS-protected CAS and BuildKit gateways."
  type        = list(string)
  default     = ["0.0.0.0/0"]

  validation {
    condition     = length(var.service_allowed_cidrs) > 0
    error_message = "Provide at least one service CIDR."
  }
}

variable "location" {
  description = "Hetzner location."
  type        = string
  default     = "fsn1"
}

variable "ssh_public_key_path" {
  description = "Absolute path to the operator SSH public key."
  type        = string
}

variable "ssh_allowed_cidrs" {
  description = "CIDRs allowed to reach SSH. Use the operator's /32, never 0.0.0.0/0."
  type        = list(string)

  validation {
    condition     = length(var.ssh_allowed_cidrs) > 0 && !contains(var.ssh_allowed_cidrs, "0.0.0.0/0")
    error_message = "Provide at least one restricted SSH CIDR; 0.0.0.0/0 is forbidden."
  }
}
