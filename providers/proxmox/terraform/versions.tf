# =============================================================================
# tdls-easy-k8s - Proxmox VE Provider Requirements
# =============================================================================

terraform {
  required_version = ">= 1.0"

  required_providers {
    proxmox = {
      source  = "bpg/proxmox"
      version = "~> 0.78"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
}

provider "proxmox" {
  # Uses PROXMOX_VE_ENDPOINT and PROXMOX_VE_API_TOKEN environment variables
}
