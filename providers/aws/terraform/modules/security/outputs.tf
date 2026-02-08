output "control_plane_sg_id" {
  description = "Control plane security group ID"
  value       = aws_security_group.control_plane.id
}

output "worker_sg_id" {
  description = "Worker security group ID"
  value       = aws_security_group.worker.id
}

output "nlb_sg_id" {
  description = "NLB security group ID (if enabled)"
  value       = var.enable_nlb ? aws_security_group.nlb[0].id : null
}

output "control_plane_sg_name" {
  description = "Control plane security group name"
  value       = aws_security_group.control_plane.name
}

output "worker_sg_name" {
  description = "Worker security group name"
  value       = aws_security_group.worker.name
}
