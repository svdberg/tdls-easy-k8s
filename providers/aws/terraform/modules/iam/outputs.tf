output "control_plane_role_arn" {
  description = "Control plane IAM role ARN"
  value       = aws_iam_role.control_plane.arn
}

output "control_plane_role_name" {
  description = "Control plane IAM role name"
  value       = aws_iam_role.control_plane.name
}

output "control_plane_instance_profile_name" {
  description = "Control plane instance profile name"
  value       = aws_iam_instance_profile.control_plane.name
}

output "control_plane_instance_profile_arn" {
  description = "Control plane instance profile ARN"
  value       = aws_iam_instance_profile.control_plane.arn
}

output "worker_role_arn" {
  description = "Worker IAM role ARN"
  value       = aws_iam_role.worker.arn
}

output "worker_role_name" {
  description = "Worker IAM role name"
  value       = aws_iam_role.worker.name
}

output "worker_instance_profile_name" {
  description = "Worker instance profile name"
  value       = aws_iam_instance_profile.worker.name
}

output "worker_instance_profile_arn" {
  description = "Worker instance profile ARN"
  value       = aws_iam_instance_profile.worker.arn
}

output "kms_key_id" {
  description = "KMS key ID (if encryption enabled)"
  value       = var.enable_encryption ? aws_kms_key.cluster[0].id : null
}

output "kms_key_arn" {
  description = "KMS key ARN (if encryption enabled)"
  value       = var.enable_encryption ? aws_kms_key.cluster[0].arn : null
}
