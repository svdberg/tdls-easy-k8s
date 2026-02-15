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

# =============================================================================
# Hetzner Configuration
# =============================================================================

variable "location" {
  description = "Hetzner Cloud location (fsn1, nbg1, hel1, ash, hil)"
  type        = string
  default     = "fsn1"

  validation {
    condition     = contains(["fsn1", "nbg1", "hel1", "ash", "hil"], var.location)
    error_message = "Location must be one of: fsn1, nbg1, hel1, ash, hil."
  }
}

# =============================================================================
# Networking Configuration
# =============================================================================

variable "network_cidr" {
  description = "CIDR block for the private network"
  type        = string
  default     = "10.0.0.0/16"
}

variable "subnet_cidr" {
  description = "CIDR block for the subnet"
  type        = string
  default     = "10.0.1.0/24"
}

# =============================================================================
# Compute Configuration - Control Plane
# =============================================================================

variable "cp_count" {
  description = "Number of control plane nodes (must be odd: 1, 3, 5 for etcd quorum)"
  type        = number
  default     = 1

  validation {
    condition     = var.cp_count % 2 == 1 && var.cp_count >= 1
    error_message = "Control plane count must be odd (1, 3, 5, etc.) for etcd quorum."
  }
}

variable "server_type_cp" {
  description = "Hetzner server type for control plane nodes (e.g., cpx21, cpx31, cx23)"
  type        = string
  default     = "cpx22"
}

# =============================================================================
# Compute Configuration - Workers
# =============================================================================

variable "worker_count" {
  description = "Number of worker nodes"
  type        = number
  default     = 2

  validation {
    condition     = var.worker_count >= 0
    error_message = "Worker count must be >= 0."
  }
}

variable "server_type_worker" {
  description = "Hetzner server type for worker nodes (e.g., cpx21, cpx31, cx23)"
  type        = string
  default     = "cpx32"
}

# =============================================================================
# OS Configuration
# =============================================================================

variable "os_image" {
  description = "OS image for servers"
  type        = string
  default     = "ubuntu-22.04"
}

# =============================================================================
# Kubernetes Configuration
# =============================================================================

variable "kubernetes_version" {
  description = "Kubernetes version (e.g., '1.30')"
  type        = string
  default     = "1.30"
}

variable "rke2_version" {
  description = "RKE2 version (leave empty for latest matching k8s version)"
  type        = string
  default     = ""
}

variable "cni_plugin" {
  description = "CNI plugin (canal, calico, cilium)"
  type        = string
  default     = "canal"

  validation {
    condition     = contains(["canal", "calico", "cilium"], var.cni_plugin)
    error_message = "CNI plugin must be one of: canal, calico, cilium."
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
