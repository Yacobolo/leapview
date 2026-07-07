output "url" {
  description = "LibreDash HTTPS URL."
  value       = "https://${local.domain}"
}

output "server_ipv4" {
  description = "Reserved public IPv4."
  value       = hcloud_primary_ip.libredash.ip_address
}

output "ssh_command" {
  description = "SSH command for retrieving first-login credentials and inspecting logs."
  value       = "ssh${local.ssh_identity_arg} root@${hcloud_primary_ip.libredash.ip_address}"
}

output "initial_local_user_command" {
  description = "Command to read the initial local user's one-time password from the server."
  value       = "ssh${local.ssh_identity_arg} root@${hcloud_primary_ip.libredash.ip_address} 'cat /root/libredash-initial-local-user.json'"
}

output "bootstrap_token_command" {
  description = "Command to read the bootstrap API token from the server."
  value       = "ssh${local.ssh_identity_arg} root@${hcloud_primary_ip.libredash.ip_address} 'cat /root/libredash-bootstrap-token'"
}
