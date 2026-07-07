variable "hcloud_token" {
  description = "Hetzner Cloud API token. Prefer HCLOUD_TOKEN or TF_VAR_hcloud_token from your shell instead of a tfvars file."
  type        = string
  sensitive   = true
  nullable    = true
  default     = null
}

variable "name" {
  description = "Name prefix for Hetzner resources."
  type        = string
  default     = "libredash"
}

variable "server_type" {
  description = "Hetzner server type. cpx22 gives enough CPU/RAM to build the image on first boot."
  type        = string
  default     = "cpx22"
}

variable "location" {
  description = "Hetzner location for the server and reserved primary IPv4."
  type        = string
  default     = "fsn1"
}

variable "image" {
  description = "Base operating-system image."
  type        = string
  default     = "ubuntu-24.04"
}

variable "ssh_allowed_cidrs" {
  description = "CIDR ranges allowed to reach SSH. Lock this down for durable deployments."
  type        = list(string)
  default     = ["0.0.0.0/0", "::/0"]
}

variable "ssh_key_ids" {
  description = "Existing Hetzner SSH key names or IDs to attach to the server."
  type        = list(string)
  default     = []
}

variable "ssh_public_key_path" {
  description = "Local SSH public key to upload as a Hetzner SSH key. Set to empty to disable."
  type        = string
  default     = "~/.ssh/id_ed25519.pub"
}

variable "domain" {
  description = "Hostname for HTTPS. Leave empty to use a reserved-IP sslip.io hostname."
  type        = string
  default     = ""
}

variable "admin_email" {
  description = "Initial platform admin and local-login email."
  type        = string
}

variable "repo_url" {
  description = "Git repository cloned on first boot to build the LibreDash image."
  type        = string
  default     = "https://github.com/Yacobolo/libredash.git"
}

variable "repo_ref" {
  description = "Git ref checked out for the first-boot Docker build."
  type        = string
  default     = "main"
}
