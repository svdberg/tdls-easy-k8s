variable "cluster_name" {
  description = "Name of the cluster"
  type        = string
}

variable "control_plane_count" {
  description = "Number of control plane nodes"
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

variable "etcd_volume_ids" {
  description = "List of etcd EBS volume IDs"
  type        = list(string)
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

variable "cni_plugin" {
  description = "CNI plugin (cilium, canal, calico)"
  type        = string
}

variable "cluster_cidr" {
  description = "Pod network CIDR"
  type        = string
}

variable "service_cidr" {
  description = "Service network CIDR"
  type        = string
}

variable "cluster_dns" {
  description = "Cluster DNS server IP"
  type        = string
}

variable "state_bucket" {
  description = "S3 bucket for cluster state"
  type        = string
}

variable "nlb_dns_name" {
  description = "NLB DNS name (empty if NLB disabled)"
  type        = string
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
