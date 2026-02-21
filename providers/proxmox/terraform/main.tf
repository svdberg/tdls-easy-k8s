# =============================================================================
# tdls-easy-k8s - Proxmox VE Main Configuration
# =============================================================================

locals {
  common_tags = ["cluster:${var.cluster_name}", "managed-by:tdls-easy-k8s"]
}

# =============================================================================
# SSH Key
# =============================================================================

resource "tls_private_key" "ssh" {
  algorithm = "ED25519"
}

# =============================================================================
# Ubuntu Cloud Image
# =============================================================================

resource "proxmox_virtual_environment_download_file" "ubuntu" {
  content_type = "iso"
  datastore_id = "local"
  node_name    = var.proxmox_node
  url          = "https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img"
  file_name    = "ubuntu-22.04-cloudimg-amd64.img"
}

# =============================================================================
# Cluster Token
# =============================================================================

resource "random_password" "cluster_token" {
  length  = 64
  special = false
}

# =============================================================================
# Cloud-Init Snippets
# =============================================================================

resource "proxmox_virtual_environment_file" "cloudinit_cp_init" {
  content_type = "snippets"
  datastore_id = var.snippets_datastore
  node_name    = var.proxmox_node

  source_raw {
    data = templatefile("${path.module}/user-data-cp.tpl", {
      cluster_name   = var.cluster_name
      cluster_token  = random_password.cluster_token.result
      rke2_version   = var.rke2_version
      cni_plugin     = var.cni_plugin
      cluster_cidr   = var.cluster_cidr
      service_cidr   = var.service_cidr
      cluster_dns    = var.cluster_dns
      vip_address    = var.vip_address
      is_first_node  = "true"
      first_node_ip  = ""
      node_index     = 0
      ssh_public_key = tls_private_key.ssh.public_key_openssh
    })
    file_name = "${var.cluster_name}-cp-init-userdata.yaml"
  }
}

resource "proxmox_virtual_environment_file" "cloudinit_cp_join" {
  count        = max(0, var.cp_count - 1)
  content_type = "snippets"
  datastore_id = var.snippets_datastore
  node_name    = var.proxmox_node

  source_raw {
    data = templatefile("${path.module}/user-data-cp.tpl", {
      cluster_name   = var.cluster_name
      cluster_token  = random_password.cluster_token.result
      rke2_version   = var.rke2_version
      cni_plugin     = var.cni_plugin
      cluster_cidr   = var.cluster_cidr
      service_cidr   = var.service_cidr
      cluster_dns    = var.cluster_dns
      vip_address    = var.vip_address
      is_first_node  = "false"
      first_node_ip  = proxmox_virtual_environment_vm.control_plane_init.ipv4_addresses[1][0]
      node_index     = count.index + 1
      ssh_public_key = tls_private_key.ssh.public_key_openssh
    })
    file_name = "${var.cluster_name}-cp-join-${count.index + 1}-userdata.yaml"
  }
}

resource "proxmox_virtual_environment_file" "cloudinit_worker" {
  count        = var.worker_count
  content_type = "snippets"
  datastore_id = var.snippets_datastore
  node_name    = var.proxmox_node

  source_raw {
    data = templatefile("${path.module}/user-data-worker.tpl", {
      cluster_name   = var.cluster_name
      cluster_token  = random_password.cluster_token.result
      rke2_version   = var.rke2_version
      vip_address    = var.vip_address
      first_node_ip  = proxmox_virtual_environment_vm.control_plane_init.ipv4_addresses[1][0]
      node_index     = count.index
      ssh_public_key = tls_private_key.ssh.public_key_openssh
    })
    file_name = "${var.cluster_name}-worker-${count.index}-userdata.yaml"
  }
}

# =============================================================================
# Control Plane - First Node (bootstraps the cluster)
# =============================================================================

resource "proxmox_virtual_environment_vm" "control_plane_init" {
  name      = "${var.cluster_name}-cp-0"
  node_name = var.proxmox_node
  tags      = local.common_tags

  agent {
    enabled = true
  }

  cpu {
    cores = var.cp_cpu
    type  = "x86-64-v2-AES"
  }

  memory {
    dedicated = var.cp_memory_mb
  }

  disk {
    datastore_id = var.datastore
    file_id      = proxmox_virtual_environment_download_file.ubuntu.id
    interface    = "virtio0"
    size         = var.cp_disk_gb
    discard      = "on"
    iothread     = true
  }

  network_device {
    bridge  = var.bridge
    vlan_id = var.vlan_tag > 0 ? var.vlan_tag : null
  }

  initialization {
    ip_config {
      ipv4 {
        address = "dhcp"
      }
    }
    user_data_file_id = proxmox_virtual_environment_file.cloudinit_cp_init.id
  }

  # Wait for QEMU guest agent to report IPs
  lifecycle {
    ignore_changes = [
      initialization[0].user_data_file_id,
    ]
  }
}

# =============================================================================
# Control Plane - Join Nodes (additional CP nodes, if any)
# =============================================================================

resource "proxmox_virtual_environment_vm" "control_plane_join" {
  count     = max(0, var.cp_count - 1)
  name      = "${var.cluster_name}-cp-${count.index + 1}"
  node_name = var.proxmox_node
  tags      = local.common_tags

  agent {
    enabled = true
  }

  cpu {
    cores = var.cp_cpu
    type  = "x86-64-v2-AES"
  }

  memory {
    dedicated = var.cp_memory_mb
  }

  disk {
    datastore_id = var.datastore
    file_id      = proxmox_virtual_environment_download_file.ubuntu.id
    interface    = "virtio0"
    size         = var.cp_disk_gb
    discard      = "on"
    iothread     = true
  }

  network_device {
    bridge  = var.bridge
    vlan_id = var.vlan_tag > 0 ? var.vlan_tag : null
  }

  initialization {
    ip_config {
      ipv4 {
        address = "dhcp"
      }
    }
    user_data_file_id = proxmox_virtual_environment_file.cloudinit_cp_join[count.index].id
  }

  depends_on = [
    proxmox_virtual_environment_vm.control_plane_init,
  ]

  lifecycle {
    ignore_changes = [
      initialization[0].user_data_file_id,
    ]
  }
}

# =============================================================================
# Worker Nodes
# =============================================================================

resource "proxmox_virtual_environment_vm" "worker" {
  count     = var.worker_count
  name      = "${var.cluster_name}-worker-${count.index}"
  node_name = var.proxmox_node
  tags      = local.common_tags

  agent {
    enabled = true
  }

  cpu {
    cores = var.worker_cpu
    type  = "x86-64-v2-AES"
  }

  memory {
    dedicated = var.worker_memory_mb
  }

  disk {
    datastore_id = var.datastore
    file_id      = proxmox_virtual_environment_download_file.ubuntu.id
    interface    = "virtio0"
    size         = var.worker_disk_gb
    discard      = "on"
    iothread     = true
  }

  network_device {
    bridge  = var.bridge
    vlan_id = var.vlan_tag > 0 ? var.vlan_tag : null
  }

  initialization {
    ip_config {
      ipv4 {
        address = "dhcp"
      }
    }
    user_data_file_id = proxmox_virtual_environment_file.cloudinit_worker[count.index].id
  }

  depends_on = [
    proxmox_virtual_environment_vm.control_plane_init,
  ]

  lifecycle {
    ignore_changes = [
      initialization[0].user_data_file_id,
    ]
  }
}
