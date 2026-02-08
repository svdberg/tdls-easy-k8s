variable "cluster_name" {
  description = "Name of the cluster"
  type        = string
}

variable "control_plane_count" {
  description = "Number of control plane nodes"
  type        = number
}

variable "availability_zones" {
  description = "List of availability zones"
  type        = list(string)
}

variable "etcd_volume_size" {
  description = "Size of EBS volume for etcd (GB)"
  type        = number
}

variable "etcd_volume_type" {
  description = "Type of EBS volume for etcd"
  type        = string
}

variable "etcd_volume_iops" {
  description = "IOPS for etcd EBS volume"
  type        = number
}

variable "etcd_volume_throughput" {
  description = "Throughput for etcd EBS volume (MB/s)"
  type        = number
}

variable "kms_key_id" {
  description = "KMS key ID for encryption (null to disable)"
  type        = string
  default     = null
}

variable "tags" {
  description = "Additional tags for resources"
  type        = map(string)
  default     = {}
}
