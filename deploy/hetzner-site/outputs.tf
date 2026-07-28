output "reserved_ipv4" {
  description = "Stable reserved IPv4 for the leapview.dev apex A record."
  value       = hcloud_primary_ip.site.ip_address
}

output "canonical_hostname" {
  description = "Canonical public hostname."
  value       = local.canonical_hostname
}

output "site_url" {
  description = "Canonical public-site URL."
  value       = "https://${local.canonical_hostname}"
}

output "deployment_target" {
  description = "Bounded deployment channel target; credentials are intentionally excluded."
  value       = "root@${hcloud_primary_ip.site.ip_address}"
}

output "dns_records" {
  description = "Records required by the reviewed DNS cutover."
  value = {
    apex = {
      name  = "@"
      type  = "A"
      value = hcloud_primary_ip.site.ip_address
    }
    www = {
      name  = "www"
      type  = "CNAME"
      value = "${local.canonical_hostname}."
    }
  }
}
