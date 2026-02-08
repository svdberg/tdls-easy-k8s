output "instance_ids" {
  description = "List of control plane instance IDs"
  value       = concat(aws_instance.control_plane_first[*].id, aws_instance.control_plane[*].id)
}

output "private_ips" {
  description = "List of control plane private IPs"
  value       = concat(aws_instance.control_plane_first[*].private_ip, aws_instance.control_plane[*].private_ip)
}

output "public_ips" {
  description = "List of control plane public IPs"
  value       = concat(aws_instance.control_plane_first[*].public_ip, aws_instance.control_plane[*].public_ip)
}

output "first_node_ip" {
  description = "First control plane node private IP"
  value       = length(aws_instance.control_plane_first) > 0 ? aws_instance.control_plane_first[0].private_ip : null
}
