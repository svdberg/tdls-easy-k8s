# =============================================================================
# Cluster Configuration
# =============================================================================

variable "cluster_name" {
  description = "Name of the Kubernetes cluster"
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9-]+$", var.cluster_name))
    error_message = "Cluster name must contain only lowercase letters, numbers, and hyphens."
  }
}

variable "environment" {
  description = "Environment (dev, staging, production)"
  type        = string
  default     = "production"

  validation {
    condition     = contains(["dev", "staging", "production"], var.environment)
    error_message = "Environment must be dev, staging, or production."
  }
}

# =============================================================================
# AWS Configuration
# =============================================================================

variable "aws_region" {
  description = "AWS region for the cluster"
  type        = string
}

variable "availability_zones" {
  description = "List of availability zones (must be 3 for HA, empty for auto-detection)"
  type        = list(string)
  default     = []

  validation {
    condition     = length(var.availability_zones) == 0 || length(var.availability_zones) == 3
    error_message = "Must specify exactly 3 availability zones or leave empty for auto-detection."
  }
}

# =============================================================================
# Networking Configuration
# =============================================================================

variable "vpc_cidr" {
  description = "CIDR block for VPC"
  type        = string
  default     = "10.0.0.0/16"

  validation {
    condition     = can(cidrhost(var.vpc_cidr, 0))
    error_message = "VPC CIDR must be a valid IPv4 CIDR block."
  }
}

variable "public_subnet_cidrs" {
  description = "CIDR blocks for public subnets (control plane)"
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]

  validation {
    condition     = length(var.public_subnet_cidrs) == 3
    error_message = "Must specify exactly 3 public subnet CIDRs for multi-AZ deployment."
  }
}

variable "private_subnet_cidrs" {
  description = "CIDR blocks for private subnets (workers)"
  type        = list(string)
  default     = ["10.0.11.0/24", "10.0.12.0/24", "10.0.13.0/24"]

  validation {
    condition     = length(var.private_subnet_cidrs) == 3
    error_message = "Must specify exactly 3 private subnet CIDRs for multi-AZ deployment."
  }
}

variable "enable_nat_gateway" {
  description = "Enable NAT Gateway for private subnets"
  type        = bool
  default     = true
}

variable "single_nat_gateway" {
  description = "Use a single NAT Gateway (cost optimization, reduces HA)"
  type        = bool
  default     = false
}

variable "enable_vpc_endpoints" {
  description = "Enable VPC endpoints for S3 and ECR"
  type        = bool
  default     = true
}

# =============================================================================
# Load Balancer Configuration
# =============================================================================

variable "enable_nlb" {
  description = "Enable Network Load Balancer for Kubernetes API"
  type        = bool
  default     = true
}

variable "nlb_internal" {
  description = "Make NLB internal (not internet-facing)"
  type        = bool
  default     = false
}

variable "api_server_allowed_cidrs" {
  description = "CIDR blocks allowed to access Kubernetes API"
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

# =============================================================================
# Compute Configuration - Control Plane
# =============================================================================

variable "control_plane_count" {
  description = "Number of control plane nodes (must be odd: 1, 3, 5 for etcd quorum)"
  type        = number
  default     = 3

  validation {
    condition     = var.control_plane_count % 2 == 1 && var.control_plane_count >= 1
    error_message = "Control plane count must be odd (1, 3, 5, etc.) for etcd quorum."
  }
}

variable "control_plane_instance_type" {
  description = "EC2 instance type for control plane nodes"
  type        = string
  default     = "t3.medium"
}

variable "control_plane_root_volume_size" {
  description = "Size of root EBS volume for control plane (GB)"
  type        = number
  default     = 50
}

variable "control_plane_root_volume_type" {
  description = "Type of root EBS volume for control plane"
  type        = string
  default     = "gp3"
}

# =============================================================================
# Compute Configuration - Workers
# =============================================================================

variable "worker_count" {
  description = "Number of worker nodes"
  type        = number
  default     = 3

  validation {
    condition     = var.worker_count >= 0
    error_message = "Worker count must be >= 0."
  }
}

variable "worker_instance_type" {
  description = "EC2 instance type for worker nodes"
  type        = string
  default     = "t3.large"
}

variable "worker_root_volume_size" {
  description = "Size of root EBS volume for workers (GB)"
  type        = number
  default     = 100
}

variable "worker_root_volume_type" {
  description = "Type of root EBS volume for workers"
  type        = string
  default     = "gp3"
}

variable "enable_spot_instances" {
  description = "Use EC2 spot instances for workers (cost optimization)"
  type        = bool
  default     = false
}

# =============================================================================
# AMI Configuration
# =============================================================================

variable "ami_id" {
  description = "AMI ID for EC2 instances (leave empty for auto-detection)"
  type        = string
  default     = ""
}

variable "ami_owner" {
  description = "AMI owner for filtering (default: Canonical/Ubuntu)"
  type        = string
  default     = "099720109477" # Canonical
}

variable "ami_name_filter" {
  description = "AMI name filter for auto-detection"
  type        = string
  default     = "ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"
}

# =============================================================================
# Kubernetes Configuration
# =============================================================================

variable "kubernetes_version" {
  description = "Kubernetes version (e.g., '1.30')"
  type        = string
}

variable "rke2_version" {
  description = "RKE2 version (e.g., 'v1.30.0+rke2r1', leave empty for latest matching k8s version)"
  type        = string
  default     = ""
}

variable "kubernetes_distribution" {
  description = "Kubernetes distribution (rke2 or k3s)"
  type        = string
  default     = "rke2"

  validation {
    condition     = contains(["rke2", "k3s"], var.kubernetes_distribution)
    error_message = "Distribution must be 'rke2' or 'k3s'."
  }
}

variable "cluster_token" {
  description = "Cluster join token (auto-generated if empty)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "cni_plugin" {
  description = "CNI plugin (cilium, canal, calico)"
  type        = string
  default     = "cilium"

  validation {
    condition     = contains(["cilium", "canal", "calico"], var.cni_plugin)
    error_message = "CNI plugin must be one of: cilium, canal, calico."
  }
}

variable "cluster_cidr" {
  description = "Pod network CIDR"
  type        = string
  default     = "10.42.0.0/16"
}

variable "service_cidr" {
  description = "Service network CIDR"
  type        = string
  default     = "10.43.0.0/16"
}

variable "cluster_dns" {
  description = "Cluster DNS server IP (usually .10 of service CIDR)"
  type        = string
  default     = "10.43.0.10"
}

# =============================================================================
# Storage Configuration
# =============================================================================

variable "etcd_volume_size" {
  description = "Size of EBS volume for etcd (GB)"
  type        = number
  default     = 50
}

variable "etcd_volume_type" {
  description = "Type of EBS volume for etcd"
  type        = string
  default     = "gp3"

  validation {
    condition     = contains(["gp3", "gp2", "io1", "io2"], var.etcd_volume_type)
    error_message = "Volume type must be one of: gp3, gp2, io1, io2."
  }
}

variable "etcd_volume_iops" {
  description = "IOPS for etcd EBS volume (gp3/io1/io2 only)"
  type        = number
  default     = 3000
}

variable "etcd_volume_throughput" {
  description = "Throughput for etcd EBS volume in MB/s (gp3 only)"
  type        = number
  default     = 125
}

# =============================================================================
# State and Backup Configuration
# =============================================================================

variable "state_bucket" {
  description = "S3 bucket for cluster state and kubeconfig (will be created if doesn't exist)"
  type        = string
}

variable "enable_etcd_backup" {
  description = "Enable automated etcd backups to S3"
  type        = bool
  default     = true
}

variable "etcd_backup_schedule" {
  description = "Cron schedule for etcd backups (e.g., '0 */6 * * *' for every 6 hours)"
  type        = string
  default     = "0 */6 * * *"
}

variable "etcd_backup_retention_days" {
  description = "Number of days to retain etcd backups"
  type        = number
  default     = 30
}

# =============================================================================
# Monitoring and Logging Configuration
# =============================================================================

variable "enable_cloudwatch_logs" {
  description = "Enable CloudWatch Logs for cluster nodes"
  type        = bool
  default     = true
}

variable "cloudwatch_log_retention_days" {
  description = "CloudWatch Logs retention in days"
  type        = number
  default     = 7

  validation {
    condition     = contains([1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653], var.cloudwatch_log_retention_days)
    error_message = "Must be a valid CloudWatch Logs retention period."
  }
}

variable "enable_metrics_server" {
  description = "Enable Kubernetes metrics server"
  type        = bool
  default     = true
}

# =============================================================================
# SSH and Access Configuration
# =============================================================================

variable "ssh_key_name" {
  description = "SSH key name for EC2 instances (leave empty to use Session Manager only)"
  type        = string
  default     = ""
}

variable "enable_session_manager" {
  description = "Enable AWS Systems Manager Session Manager for secure shell access"
  type        = bool
  default     = true
}

# =============================================================================
# Security Configuration
# =============================================================================

variable "enable_encryption" {
  description = "Enable encryption for EBS volumes and S3 buckets"
  type        = bool
  default     = true
}

variable "kms_key_deletion_window" {
  description = "KMS key deletion window in days"
  type        = number
  default     = 30

  validation {
    condition     = var.kms_key_deletion_window >= 7 && var.kms_key_deletion_window <= 30
    error_message = "KMS key deletion window must be between 7 and 30 days."
  }
}

# =============================================================================
# Tags
# =============================================================================

variable "additional_tags" {
  description = "Additional tags for all resources"
  type        = map(string)
  default     = {}
}
