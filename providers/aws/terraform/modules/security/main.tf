# =============================================================================
# Control Plane Security Group
# =============================================================================

resource "aws_security_group" "control_plane" {
  name_prefix = "${var.cluster_name}-control-plane-"
  description = "Security group for Kubernetes control plane nodes"
  vpc_id      = var.vpc_id

  tags = merge(
    {
      Name                                        = "${var.cluster_name}-control-plane-sg"
      "kubernetes.io/cluster/${var.cluster_name}" = "owned"
    },
    var.tags
  )

  lifecycle {
    create_before_destroy = true
  }
}

# Kubernetes API Server (from NLB or allowed CIDRs)
resource "aws_security_group_rule" "control_plane_api_ingress" {
  type              = "ingress"
  from_port         = 6443
  to_port           = 6443
  protocol          = "tcp"
  cidr_blocks       = var.api_server_allowed_cidrs
  description       = "Kubernetes API server"
  security_group_id = aws_security_group.control_plane.id
}

# RKE2 Supervisor/Registration API (from control plane, workers, and NLB)
resource "aws_security_group_rule" "control_plane_supervisor_ingress" {
  type              = "ingress"
  from_port         = 9345
  to_port           = 9345
  protocol          = "tcp"
  cidr_blocks       = [var.vpc_cidr]
  description       = "RKE2 supervisor API (workers, NLB, control plane)"
  security_group_id = aws_security_group.control_plane.id
}

# etcd client port (from other control plane nodes)
resource "aws_security_group_rule" "control_plane_etcd_client_ingress" {
  type                     = "ingress"
  from_port                = 2379
  to_port                  = 2379
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.control_plane.id
  description              = "etcd client"
  security_group_id        = aws_security_group.control_plane.id
}

# etcd peer port (from other control plane nodes)
resource "aws_security_group_rule" "control_plane_etcd_peer_ingress" {
  type                     = "ingress"
  from_port                = 2380
  to_port                  = 2380
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.control_plane.id
  description              = "etcd peer"
  security_group_id        = aws_security_group.control_plane.id
}

# Kubelet API (from control plane and workers)
resource "aws_security_group_rule" "control_plane_kubelet_ingress_self" {
  type                     = "ingress"
  from_port                = 10250
  to_port                  = 10250
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.control_plane.id
  description              = "Kubelet API (from control plane)"
  security_group_id        = aws_security_group.control_plane.id
}

resource "aws_security_group_rule" "control_plane_kubelet_ingress_workers" {
  type                     = "ingress"
  from_port                = 10250
  to_port                  = 10250
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.worker.id
  description              = "Kubelet API (from workers)"
  security_group_id        = aws_security_group.control_plane.id
}

# SSH (for emergency access or Session Manager)
resource "aws_security_group_rule" "control_plane_ssh_ingress" {
  type              = "ingress"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = var.api_server_allowed_cidrs
  description       = "SSH access"
  security_group_id = aws_security_group.control_plane.id
}

# VXLAN for CNI (Cilium/Flannel)
resource "aws_security_group_rule" "control_plane_vxlan_ingress" {
  type                     = "ingress"
  from_port                = 8472
  to_port                  = 8472
  protocol                 = "udp"
  source_security_group_id = aws_security_group.control_plane.id
  description              = "VXLAN overlay (from control plane)"
  security_group_id        = aws_security_group.control_plane.id
}

resource "aws_security_group_rule" "control_plane_vxlan_ingress_workers" {
  type                     = "ingress"
  from_port                = 8472
  to_port                  = 8472
  protocol                 = "udp"
  source_security_group_id = aws_security_group.worker.id
  description              = "VXLAN overlay (from workers)"
  security_group_id        = aws_security_group.control_plane.id
}

# Alternative VXLAN port
resource "aws_security_group_rule" "control_plane_vxlan_alt_ingress" {
  type                     = "ingress"
  from_port                = 4789
  to_port                  = 4789
  protocol                 = "udp"
  source_security_group_id = aws_security_group.control_plane.id
  description              = "VXLAN overlay alt port (from control plane)"
  security_group_id        = aws_security_group.control_plane.id
}

resource "aws_security_group_rule" "control_plane_vxlan_alt_ingress_workers" {
  type                     = "ingress"
  from_port                = 4789
  to_port                  = 4789
  protocol                 = "udp"
  source_security_group_id = aws_security_group.worker.id
  description              = "VXLAN overlay alt port (from workers)"
  security_group_id        = aws_security_group.control_plane.id
}

# Wireguard for CNI (Cilium)
resource "aws_security_group_rule" "control_plane_wireguard_ingress" {
  type                     = "ingress"
  from_port                = 51871
  to_port                  = 51871
  protocol                 = "udp"
  source_security_group_id = aws_security_group.control_plane.id
  description              = "WireGuard for Cilium (from control plane)"
  security_group_id        = aws_security_group.control_plane.id
}

resource "aws_security_group_rule" "control_plane_wireguard_ingress_workers" {
  type                     = "ingress"
  from_port                = 51871
  to_port                  = 51871
  protocol                 = "udp"
  source_security_group_id = aws_security_group.worker.id
  description              = "WireGuard for Cilium (from workers)"
  security_group_id        = aws_security_group.control_plane.id
}

# Allow all outbound traffic
resource "aws_security_group_rule" "control_plane_egress" {
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  description       = "Allow all outbound"
  security_group_id = aws_security_group.control_plane.id
}

# =============================================================================
# Worker Security Group
# =============================================================================

resource "aws_security_group" "worker" {
  name_prefix = "${var.cluster_name}-worker-"
  description = "Security group for Kubernetes worker nodes"
  vpc_id      = var.vpc_id

  tags = merge(
    {
      Name                                        = "${var.cluster_name}-worker-sg"
      "kubernetes.io/cluster/${var.cluster_name}" = "owned"
    },
    var.tags
  )

  lifecycle {
    create_before_destroy = true
  }
}

# Kubelet API (from control plane and other workers)
resource "aws_security_group_rule" "worker_kubelet_ingress_control_plane" {
  type                     = "ingress"
  from_port                = 10250
  to_port                  = 10250
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.control_plane.id
  description              = "Kubelet API (from control plane)"
  security_group_id        = aws_security_group.worker.id
}

resource "aws_security_group_rule" "worker_kubelet_ingress_self" {
  type                     = "ingress"
  from_port                = 10250
  to_port                  = 10250
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.worker.id
  description              = "Kubelet API (from workers)"
  security_group_id        = aws_security_group.worker.id
}

# NodePort Services (30000-32767)
resource "aws_security_group_rule" "worker_nodeport_ingress" {
  type              = "ingress"
  from_port         = 30000
  to_port           = 32767
  protocol          = "tcp"
  cidr_blocks       = [var.vpc_cidr]
  description       = "NodePort services"
  security_group_id = aws_security_group.worker.id
}

# VXLAN for CNI
resource "aws_security_group_rule" "worker_vxlan_ingress_control_plane" {
  type                     = "ingress"
  from_port                = 8472
  to_port                  = 8472
  protocol                 = "udp"
  source_security_group_id = aws_security_group.control_plane.id
  description              = "VXLAN overlay (from control plane)"
  security_group_id        = aws_security_group.worker.id
}

resource "aws_security_group_rule" "worker_vxlan_ingress_self" {
  type                     = "ingress"
  from_port                = 8472
  to_port                  = 8472
  protocol                 = "udp"
  source_security_group_id = aws_security_group.worker.id
  description              = "VXLAN overlay (from workers)"
  security_group_id        = aws_security_group.worker.id
}

# Alternative VXLAN port
resource "aws_security_group_rule" "worker_vxlan_alt_ingress_control_plane" {
  type                     = "ingress"
  from_port                = 4789
  to_port                  = 4789
  protocol                 = "udp"
  source_security_group_id = aws_security_group.control_plane.id
  description              = "VXLAN overlay alt port (from control plane)"
  security_group_id        = aws_security_group.worker.id
}

resource "aws_security_group_rule" "worker_vxlan_alt_ingress_self" {
  type                     = "ingress"
  from_port                = 4789
  to_port                  = 4789
  protocol                 = "udp"
  source_security_group_id = aws_security_group.worker.id
  description              = "VXLAN overlay alt port (from workers)"
  security_group_id        = aws_security_group.worker.id
}

# Wireguard for CNI (Cilium)
resource "aws_security_group_rule" "worker_wireguard_ingress_control_plane" {
  type                     = "ingress"
  from_port                = 51871
  to_port                  = 51871
  protocol                 = "udp"
  source_security_group_id = aws_security_group.control_plane.id
  description              = "WireGuard for Cilium (from control plane)"
  security_group_id        = aws_security_group.worker.id
}

resource "aws_security_group_rule" "worker_wireguard_ingress_self" {
  type                     = "ingress"
  from_port                = 51871
  to_port                  = 51871
  protocol                 = "udp"
  source_security_group_id = aws_security_group.worker.id
  description              = "WireGuard for Cilium (from workers)"
  security_group_id        = aws_security_group.worker.id
}

# SSH (for emergency access or Session Manager)
resource "aws_security_group_rule" "worker_ssh_ingress" {
  type              = "ingress"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = [var.vpc_cidr]
  description       = "SSH access from VPC"
  security_group_id = aws_security_group.worker.id
}

# Allow all outbound traffic
resource "aws_security_group_rule" "worker_egress" {
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  description       = "Allow all outbound"
  security_group_id = aws_security_group.worker.id
}

# =============================================================================
# Network Load Balancer Security Group
# =============================================================================

resource "aws_security_group" "nlb" {
  count = var.enable_nlb ? 1 : 0

  name_prefix = "${var.cluster_name}-nlb-"
  description = "Security group for Network Load Balancer"
  vpc_id      = var.vpc_id

  tags = merge(
    {
      Name = "${var.cluster_name}-nlb-sg"
    },
    var.tags
  )

  lifecycle {
    create_before_destroy = true
  }
}

# Kubernetes API (from allowed CIDRs)
resource "aws_security_group_rule" "nlb_api_ingress" {
  count = var.enable_nlb ? 1 : 0

  type              = "ingress"
  from_port         = 6443
  to_port           = 6443
  protocol          = "tcp"
  cidr_blocks       = var.api_server_allowed_cidrs
  description       = "Kubernetes API"
  security_group_id = aws_security_group.nlb[0].id
}

# Allow outbound to control plane
resource "aws_security_group_rule" "nlb_egress" {
  count = var.enable_nlb ? 1 : 0

  type                     = "egress"
  from_port                = 6443
  to_port                  = 6443
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.control_plane.id
  description              = "Forward to control plane"
  security_group_id        = aws_security_group.nlb[0].id
}
