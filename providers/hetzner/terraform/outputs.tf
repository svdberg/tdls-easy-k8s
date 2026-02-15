# =============================================================================
# tdls-easy-k8s - Hetzner Cloud Outputs
# =============================================================================

output "lb_ipv4" {
  description = "Load balancer IPv4 address (Kubernetes API endpoint)"
  value       = hcloud_load_balancer.api.ipv4
}

output "first_cp_ip" {
  description = "First control plane node public IP"
  value       = hcloud_server.control_plane_init.ipv4_address
}

output "control_plane_ips" {
  description = "Control plane node public IPs"
  value       = concat(
    [hcloud_server.control_plane_init.ipv4_address],
    hcloud_server.control_plane_join[*].ipv4_address
  )
}

output "worker_ips" {
  description = "Worker node public IPs"
  value       = hcloud_server.worker[*].ipv4_address
}

output "ssh_private_key" {
  description = "SSH private key for accessing nodes"
  value       = tls_private_key.ssh.private_key_openssh
  sensitive   = true
}

output "kubernetes_api_endpoint" {
  description = "Kubernetes API endpoint via load balancer"
  value       = "https://${hcloud_load_balancer.api.ipv4}:6443"
}
