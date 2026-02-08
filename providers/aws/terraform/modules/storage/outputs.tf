output "etcd_volume_ids" {
  description = "List of etcd EBS volume IDs"
  value       = aws_ebs_volume.etcd[*].id
}

output "etcd_volume_arns" {
  description = "List of etcd EBS volume ARNs"
  value       = aws_ebs_volume.etcd[*].arn
}
