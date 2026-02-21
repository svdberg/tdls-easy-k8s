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
# Proxmox Configuration
# =============================================================================

variable "proxmox_node" {
  description = "Proxmox VE node name where VMs will be created (e.g. 'pve')"
  type        = string
}

variable "bridge" {
  description = "Network bridge for VM NICs (e.g. 'vmbr0')"
  type        = string
  default     = "vmbr0"
}

variable "vlan_tag" {
  description = "Optional VLAN tag for VM network interfaces (0 = no VLAN)"
  type        = number
  default     = 0
}

variable "datastore" {
  description = "Storage datastore for VM disks (e.g. 'local-lvm')"
  type        = string
  default     = "local-lvm"
}

variable "snippets_datastore" {
  description = "Datastore for cloud-init snippets (must have 'Snippets' content type enabled)"
  type        = string
  default     = "local"
}

# =============================================================================
# kube-vip Configuration
# =============================================================================

variable "vip_address" {
  description = "Virtual IP address for kube-vip (must be a free IP on the network)"
  type        = string
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

variable "cp_cpu" {
  description = "Number of CPU cores for control plane nodes"
  type        = number
  default     = 4
}

variable "cp_memory_mb" {
  description = "Memory in MB for control plane nodes"
  type        = number
  default     = 8192
}

variable "cp_disk_gb" {
  description = "Disk size in GB for control plane nodes"
  type        = number
  default     = 50
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

variable "worker_cpu" {
  description = "Number of CPU cores for worker nodes"
  type        = number
  default     = 4
}

variable "worker_memory_mb" {
  description = "Memory in MB for worker nodes"
  type        = number
  default     = 8192
}

variable "worker_disk_gb" {
  description = "Disk size in GB for worker nodes"
  type        = number
  default     = 100
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
