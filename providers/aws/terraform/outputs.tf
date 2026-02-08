# =============================================================================
# Cluster Endpoint and Access
# =============================================================================

output "kubernetes_api_endpoint" {
  description = "Kubernetes API server endpoint"
  value       = local.api_endpoint
}

output "kubeconfig_s3_path" {
  description = "S3 path to kubeconfig file"
  value       = "s3://${var.state_bucket}/kubeconfig/${var.cluster_name}/rke2.yaml"
}

output "kubeconfig_download_command" {
  description = "Command to download and configure kubeconfig"
  value       = "aws s3 cp s3://${var.state_bucket}/kubeconfig/${var.cluster_name}/rke2.yaml ./kubeconfig && sed -i.bak 's/127\\.0\\.0\\.1/${local.api_endpoint}/g' ./kubeconfig && export KUBECONFIG=./kubeconfig"
}

# =============================================================================
# Networking Outputs
# =============================================================================

output "vpc_id" {
  description = "VPC ID"
  value       = module.networking.vpc_id
}

output "vpc_cidr" {
  description = "VPC CIDR block"
  value       = module.networking.vpc_cidr
}

output "public_subnet_ids" {
  description = "Public subnet IDs"
  value       = module.networking.public_subnet_ids
}

output "private_subnet_ids" {
  description = "Private subnet IDs"
  value       = module.networking.private_subnet_ids
}

output "nat_gateway_public_ips" {
  description = "NAT Gateway public IPs"
  value       = module.networking.nat_gateway_public_ips
}

# =============================================================================
# Control Plane Outputs
# =============================================================================

output "control_plane_instance_ids" {
  description = "Control plane instance IDs"
  value       = module.control_plane.instance_ids
}

output "control_plane_private_ips" {
  description = "Control plane private IPs"
  value       = module.control_plane.private_ips
}

output "control_plane_public_ips" {
  description = "Control plane public IPs"
  value       = module.control_plane.public_ips
}

# =============================================================================
# Worker Outputs
# =============================================================================

output "worker_instance_ids" {
  description = "Worker instance IDs"
  value       = module.worker.instance_ids
}

output "worker_private_ips" {
  description = "Worker private IPs"
  value       = module.worker.private_ips
}

# =============================================================================
# Load Balancer Outputs
# =============================================================================

output "nlb_arn" {
  description = "NLB ARN (if enabled)"
  value       = var.enable_nlb ? module.loadbalancer[0].nlb_arn : null
}

output "nlb_dns_name" {
  description = "NLB DNS name (if enabled)"
  value       = var.enable_nlb ? module.loadbalancer[0].nlb_dns_name : null
}

output "nlb_zone_id" {
  description = "NLB Route53 zone ID (if enabled)"
  value       = var.enable_nlb ? module.loadbalancer[0].nlb_zone_id : null
}

# =============================================================================
# Security Outputs
# =============================================================================

output "control_plane_security_group_id" {
  description = "Control plane security group ID"
  value       = module.security.control_plane_sg_id
}

output "worker_security_group_id" {
  description = "Worker security group ID"
  value       = module.security.worker_sg_id
}

# =============================================================================
# IAM Outputs
# =============================================================================

output "control_plane_iam_role_arn" {
  description = "Control plane IAM role ARN"
  value       = module.iam.control_plane_role_arn
}

output "worker_iam_role_arn" {
  description = "Worker IAM role ARN"
  value       = module.iam.worker_role_arn
}

output "kms_key_id" {
  description = "KMS key ID (if encryption enabled)"
  value       = module.iam.kms_key_id
}

# =============================================================================
# Cluster Information Summary
# =============================================================================

output "cluster_info" {
  description = "Complete cluster information"
  value = {
    name                = var.cluster_name
    region              = var.aws_region
    kubernetes_version  = var.kubernetes_version
    rke2_version        = var.rke2_version
    control_plane_count = var.control_plane_count
    worker_count        = var.worker_count
    api_endpoint        = local.api_endpoint
    cni_plugin          = var.cni_plugin
    load_balancer       = var.enable_nlb ? "AWS NLB" : "Disabled"
    vpc_id              = module.networking.vpc_id
    availability_zones  = local.availability_zones
  }
}

# =============================================================================
# Connection Commands
# =============================================================================

output "ssh_to_control_plane" {
  description = "SSH command to connect to first control plane node"
  value = var.ssh_key_name != "" ? (
    length(module.control_plane.public_ips) > 0 ? (
      "ssh -i ~/.ssh/${var.ssh_key_name}.pem ubuntu@${module.control_plane.public_ips[0]}"
    ) : "Control plane instances don't have public IPs"
  ) : "Use AWS Systems Manager Session Manager: aws ssm start-session --target ${module.control_plane.instance_ids[0]}"
}

output "test_cluster_command" {
  description = "Command to test cluster connectivity"
  value       = "aws s3 cp s3://${var.state_bucket}/kubeconfig/${var.cluster_name}/rke2.yaml ./kubeconfig && export KUBECONFIG=./kubeconfig && kubectl get nodes"
}
