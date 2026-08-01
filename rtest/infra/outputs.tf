output "server_ipv4" {
  description = "Public IPv4 for the rtest HTTPS and mTLS service endpoints."
  value       = hcloud_server.runner.ipv4_address
}

output "server_name" {
  value = hcloud_server.runner.name
}
