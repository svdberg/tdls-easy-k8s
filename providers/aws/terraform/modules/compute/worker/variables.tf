variable "cluster_name" {
  description = "Name of the cluster"
  type        = string
}

variable "worker_count" {
  description = "Number of worker nodes"
  type        = number
}

variable "instance_type" {
  description = "EC2 instance type"
  type        = string
}

variable "ami_id" {
  description = "AMI ID for instances"
  type        = string
}

variable "subnet_ids" {
  description = "List of subnet IDs"
  type        = list(string)
}

variable "security_group_ids" {
  description = "List of security group IDs"
  type        = list(string)
}

variable "iam_instance_profile_name" {
  description = "IAM instance profile name"
  type        = string
}

variable "ssh_key_name" {
  description = "SSH key name (empty for no key)"
  type        = string
}

variable "root_volume_size" {
  description = "Size of root volume (GB)"
  type        = number
}

variable "root_volume_type" {
  description = "Type of root volume"
  type        = string
}

variable "cluster_token" {
  description = "Cluster join token"
  type        = string
  sensitive   = true
}

variable "rke2_version" {
  description = "RKE2 version"
  type        = string
}

variable "api_endpoint" {
  description = "Kubernetes API endpoint (NLB DNS or first control plane IP)"
  type        = string
}

variable "enable_spot_instances" {
  description = "Use EC2 spot instances"
  type        = bool
}

variable "enable_encryption" {
  description = "Enable EBS encryption"
  type        = bool
}

variable "kms_key_id" {
  description = "KMS key ID for encryption"
  type        = string
  default     = null
}

variable "tags" {
  description = "Additional tags for resources"
  type        = map(string)
  default     = {}
}
