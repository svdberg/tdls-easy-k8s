variable "cluster_name" {
  description = "Name of the cluster"
  type        = string
}

variable "state_bucket" {
  description = "S3 bucket for cluster state"
  type        = string
}

variable "enable_encryption" {
  description = "Enable KMS encryption"
  type        = bool
}

variable "kms_key_deletion_window" {
  description = "KMS key deletion window in days"
  type        = number
}

variable "enable_session_manager" {
  description = "Enable AWS Systems Manager Session Manager"
  type        = bool
}

variable "enable_cloudwatch_logs" {
  description = "Enable CloudWatch Logs"
  type        = bool
}

variable "tags" {
  description = "Additional tags for resources"
  type        = map(string)
  default     = {}
}
