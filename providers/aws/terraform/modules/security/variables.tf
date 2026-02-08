variable "cluster_name" {
  description = "Name of the cluster"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID"
  type        = string
}

variable "vpc_cidr" {
  description = "VPC CIDR block"
  type        = string
}

variable "api_server_allowed_cidrs" {
  description = "CIDR blocks allowed to access Kubernetes API"
  type        = list(string)
}

variable "enable_nlb" {
  description = "Enable Network Load Balancer security group"
  type        = bool
}

variable "tags" {
  description = "Additional tags for resources"
  type        = map(string)
  default     = {}
}
