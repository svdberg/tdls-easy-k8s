variable "cluster_name" {
  description = "Name of the cluster"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID"
  type        = string
}

variable "subnet_ids" {
  description = "List of subnet IDs for NLB"
  type        = list(string)
}

variable "control_plane_instance_ids" {
  description = "List of control plane instance IDs"
  type        = list(string)
}

variable "nlb_internal" {
  description = "Make NLB internal (not internet-facing)"
  type        = bool
}

variable "enable_ingress" {
  description = "Enable internet-facing NLB for ingress traffic (HTTP/HTTPS)"
  type        = bool
  default     = false
}

variable "worker_instance_ids" {
  description = "List of worker instance IDs (for ingress NLB targets)"
  type        = list(string)
  default     = []
}

variable "tags" {
  description = "Additional tags for resources"
  type        = map(string)
  default     = {}
}
