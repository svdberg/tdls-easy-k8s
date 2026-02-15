# =============================================================================
# tdls-easy-k8s - Hetzner Cloud Main Configuration
# =============================================================================

locals {
  common_labels = {
    cluster    = var.cluster_name
    managed_by = "tdls-easy-k8s"
  }
}

# =============================================================================
# SSH Key
# =============================================================================

resource "tls_private_key" "ssh" {
  algorithm = "ED25519"
}

resource "hcloud_ssh_key" "cluster" {
  name       = "${var.cluster_name}-key"
  public_key = tls_private_key.ssh.public_key_openssh
  labels     = local.common_labels
}

# =============================================================================
# Network
# =============================================================================

resource "hcloud_network" "cluster" {
  name     = "${var.cluster_name}-network"
  ip_range = var.network_cidr
  labels   = local.common_labels
}

resource "hcloud_network_subnet" "cluster" {
  network_id   = hcloud_network.cluster.id
  type         = "cloud"
  network_zone = lookup({
    "fsn1" = "eu-central"
    "nbg1" = "eu-central"
    "hel1" = "eu-central"
    "ash"  = "us-east"
    "hil"  = "us-west"
  }, var.location, "eu-central")
  ip_range = var.subnet_cidr
}

# =============================================================================
# Firewall
# =============================================================================

resource "hcloud_firewall" "cluster" {
  name   = "${var.cluster_name}-firewall"
  labels = local.common_labels

  # SSH access
  rule {
    direction = "in"
    protocol  = "tcp"
    port      = "22"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  # Kubernetes API
  rule {
    direction = "in"
    protocol  = "tcp"
    port      = "6443"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  # RKE2 supervisor (for node join)
  rule {
    direction = "in"
    protocol  = "tcp"
    port      = "9345"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  # HTTP ingress
  rule {
    direction = "in"
    protocol  = "tcp"
    port      = "80"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  # HTTPS ingress
  rule {
    direction = "in"
    protocol  = "tcp"
    port      = "443"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  # Kubelet
  rule {
    direction = "in"
    protocol  = "tcp"
    port      = "10250"
    source_ips = [var.network_cidr]
  }

  # etcd (control plane only, internal)
  rule {
    direction = "in"
    protocol  = "tcp"
    port      = "2379-2381"
    source_ips = [var.network_cidr]
  }

  # VXLAN (Canal CNI)
  rule {
    direction = "in"
    protocol  = "udp"
    port      = "8472"
    source_ips = [var.network_cidr]
  }

  # NodePort range
  rule {
    direction = "in"
    protocol  = "tcp"
    port      = "30000-32767"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
}

# =============================================================================
# Load Balancer (created before servers so IP is available for TLS SANs)
# =============================================================================

resource "hcloud_load_balancer" "api" {
  name               = "${var.cluster_name}-api-lb"
  load_balancer_type = "lb11"
  location           = var.location
  labels             = local.common_labels
}

resource "hcloud_load_balancer_network" "api" {
  load_balancer_id = hcloud_load_balancer.api.id
  network_id       = hcloud_network.cluster.id

  depends_on = [hcloud_network_subnet.cluster]
}

# =============================================================================
# Cluster Token
# =============================================================================

resource "random_password" "cluster_token" {
  length  = 64
  special = false
}

# =============================================================================
# Control Plane - First Node (bootstraps the cluster)
# =============================================================================

resource "hcloud_server" "control_plane_init" {
  name        = "${var.cluster_name}-cp-0"
  server_type = var.server_type_cp
  image       = var.os_image
  location    = var.location
  ssh_keys    = [hcloud_ssh_key.cluster.id]
  labels      = merge(local.common_labels, { role = "control-plane" })

  firewall_ids = [hcloud_firewall.cluster.id]

  user_data = templatefile("${path.module}/user-data-cp.tpl", {
    cluster_name  = var.cluster_name
    cluster_token = random_password.cluster_token.result
    rke2_version  = var.rke2_version
    cni_plugin    = var.cni_plugin
    cluster_cidr  = var.cluster_cidr
    service_cidr  = var.service_cidr
    cluster_dns   = var.cluster_dns
    lb_ipv4       = hcloud_load_balancer.api.ipv4
    is_first_node = "true"
    first_node_ip = ""
    node_index    = 0
  })

  network {
    network_id = hcloud_network.cluster.id
  }

  public_net {
    ipv4_enabled = true
    ipv6_enabled = false
  }

  depends_on = [
    hcloud_network_subnet.cluster,
    hcloud_load_balancer.api,
  ]
}

# =============================================================================
# Control Plane - Join Nodes (additional CP nodes, if any)
# =============================================================================

resource "hcloud_server" "control_plane_join" {
  count       = max(0, var.cp_count - 1)
  name        = "${var.cluster_name}-cp-${count.index + 1}"
  server_type = var.server_type_cp
  image       = var.os_image
  location    = var.location
  ssh_keys    = [hcloud_ssh_key.cluster.id]
  labels      = merge(local.common_labels, { role = "control-plane" })

  firewall_ids = [hcloud_firewall.cluster.id]

  user_data = templatefile("${path.module}/user-data-cp.tpl", {
    cluster_name  = var.cluster_name
    cluster_token = random_password.cluster_token.result
    rke2_version  = var.rke2_version
    cni_plugin    = var.cni_plugin
    cluster_cidr  = var.cluster_cidr
    service_cidr  = var.service_cidr
    cluster_dns   = var.cluster_dns
    lb_ipv4       = hcloud_load_balancer.api.ipv4
    is_first_node = "false"
    first_node_ip = hcloud_server.control_plane_init.ipv4_address
    node_index    = count.index + 1
  })

  network {
    network_id = hcloud_network.cluster.id
  }

  public_net {
    ipv4_enabled = true
    ipv6_enabled = false
  }

  depends_on = [
    hcloud_network_subnet.cluster,
    hcloud_server.control_plane_init,
  ]
}

# =============================================================================
# Worker Servers
# =============================================================================

resource "hcloud_server" "worker" {
  count       = var.worker_count
  name        = "${var.cluster_name}-worker-${count.index}"
  server_type = var.server_type_worker
  image       = var.os_image
  location    = var.location
  ssh_keys    = [hcloud_ssh_key.cluster.id]
  labels      = merge(local.common_labels, { role = "worker" })

  firewall_ids = [hcloud_firewall.cluster.id]

  user_data = templatefile("${path.module}/user-data-worker.tpl", {
    cluster_name  = var.cluster_name
    cluster_token = random_password.cluster_token.result
    rke2_version  = var.rke2_version
    api_endpoint  = hcloud_server.control_plane_init.ipv4_address
    node_index    = count.index
  })

  network {
    network_id = hcloud_network.cluster.id
  }

  public_net {
    ipv4_enabled = true
    ipv6_enabled = false
  }

  depends_on = [
    hcloud_network_subnet.cluster,
    hcloud_server.control_plane_init,
  ]
}

# =============================================================================
# Load Balancer Targets & Services
# =============================================================================

resource "hcloud_load_balancer_target" "cp_init" {
  type             = "server"
  load_balancer_id = hcloud_load_balancer.api.id
  server_id        = hcloud_server.control_plane_init.id
  use_private_ip   = true

  depends_on = [hcloud_load_balancer_network.api]
}

resource "hcloud_load_balancer_target" "cp_join" {
  count            = max(0, var.cp_count - 1)
  type             = "server"
  load_balancer_id = hcloud_load_balancer.api.id
  server_id        = hcloud_server.control_plane_join[count.index].id
  use_private_ip   = true

  depends_on = [hcloud_load_balancer_network.api]
}

resource "hcloud_load_balancer_service" "api" {
  load_balancer_id = hcloud_load_balancer.api.id
  protocol         = "tcp"
  listen_port      = 6443
  destination_port = 6443

  health_check {
    protocol = "tcp"
    port     = 6443
    interval = 10
    timeout  = 5
    retries  = 3
  }

  depends_on = [hcloud_load_balancer_target.cp_init]
}

resource "hcloud_load_balancer_service" "rke2" {
  load_balancer_id = hcloud_load_balancer.api.id
  protocol         = "tcp"
  listen_port      = 9345
  destination_port = 9345

  health_check {
    protocol = "tcp"
    port     = 9345
    interval = 10
    timeout  = 5
    retries  = 3
  }

  depends_on = [hcloud_load_balancer_target.cp_init]
}
