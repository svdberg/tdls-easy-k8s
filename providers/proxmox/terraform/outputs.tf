# =============================================================================
# tdls-easy-k8s - Proxmox VE Outputs
# =============================================================================

output "vip_address" {
  description = "kube-vip virtual IP address (Kubernetes API endpoint)"
  value       = var.vip_address
}

output "first_cp_ip" {
  description = "First control plane node IP (used for SSH kubeconfig retrieval)"
  value       = proxmox_virtual_environment_vm.control_plane_init.ipv4_addresses[1][0]
}

output "control_plane_ips" {
  description = "Control plane node IPs"
  value = concat(
    [proxmox_virtual_environment_vm.control_plane_init.ipv4_addresses[1][0]],
    [for vm in proxmox_virtual_environment_vm.control_plane_join : vm.ipv4_addresses[1][0]]
  )
}

output "worker_ips" {
  description = "Worker node IPs"
  value       = [for vm in proxmox_virtual_environment_vm.worker : vm.ipv4_addresses[1][0]]
}

output "ssh_private_key" {
  description = "SSH private key for accessing nodes"
  value       = tls_private_key.ssh.private_key_openssh
  sensitive   = true
}

output "kubernetes_api_endpoint" {
  description = "Kubernetes API endpoint via kube-vip"
  value       = "https://${var.vip_address}:6443"
}
