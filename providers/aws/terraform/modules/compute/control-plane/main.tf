# =============================================================================
# Control Plane EC2 Instances
# =============================================================================

# First control plane node (leader)
resource "aws_instance" "control_plane_first" {
  count = var.control_plane_count > 0 ? 1 : 0

  ami                    = var.ami_id
  instance_type          = var.instance_type
  subnet_id              = var.subnet_ids[0 % length(var.subnet_ids)]
  vpc_security_group_ids = var.security_group_ids
  iam_instance_profile   = var.iam_instance_profile_name
  key_name               = var.ssh_key_name != "" ? var.ssh_key_name : null

  root_block_device {
    volume_size           = var.root_volume_size
    volume_type           = var.root_volume_type
    encrypted             = var.enable_encryption
    kms_key_id            = var.kms_key_id
    delete_on_termination = true

    tags = merge(
      {
        Name = "${var.cluster_name}-control-plane-0-root"
      },
      var.tags
    )
  }

  user_data = base64encode(templatefile("${path.module}/user-data.tpl", {
    cluster_name  = var.cluster_name
    cluster_token = var.cluster_token
    rke2_version  = var.rke2_version
    cni_plugin    = var.cni_plugin
    cluster_cidr  = var.cluster_cidr
    service_cidr  = var.service_cidr
    cluster_dns   = var.cluster_dns
    state_bucket  = var.state_bucket
    nlb_dns_name  = var.nlb_dns_name
    is_first_node = "true"
    first_node_ip = ""
    node_index    = 0
  }))

  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
  }

  tags = merge(
    {
      Name                                        = "${var.cluster_name}-control-plane-0"
      Role                                        = "control-plane"
      "kubernetes.io/cluster/${var.cluster_name}" = "owned"
    },
    var.tags
  )

  lifecycle {
    ignore_changes = [ami]
  }
}

# Additional control plane nodes (followers)
resource "aws_instance" "control_plane" {
  count = var.control_plane_count > 1 ? var.control_plane_count - 1 : 0

  ami                    = var.ami_id
  instance_type          = var.instance_type
  subnet_id              = var.subnet_ids[(count.index + 1) % length(var.subnet_ids)]
  vpc_security_group_ids = var.security_group_ids
  iam_instance_profile   = var.iam_instance_profile_name
  key_name               = var.ssh_key_name != "" ? var.ssh_key_name : null

  root_block_device {
    volume_size           = var.root_volume_size
    volume_type           = var.root_volume_type
    encrypted             = var.enable_encryption
    kms_key_id            = var.kms_key_id
    delete_on_termination = true

    tags = merge(
      {
        Name = "${var.cluster_name}-control-plane-${count.index + 1}-root"
      },
      var.tags
    )
  }

  user_data = base64encode(templatefile("${path.module}/user-data.tpl", {
    cluster_name  = var.cluster_name
    cluster_token = var.cluster_token
    rke2_version  = var.rke2_version
    cni_plugin    = var.cni_plugin
    cluster_cidr  = var.cluster_cidr
    service_cidr  = var.service_cidr
    cluster_dns   = var.cluster_dns
    state_bucket  = var.state_bucket
    nlb_dns_name  = var.nlb_dns_name
    is_first_node = "false"
    first_node_ip = aws_instance.control_plane_first[0].private_ip
    node_index    = count.index + 1
  }))

  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
  }

  tags = merge(
    {
      Name                                        = "${var.cluster_name}-control-plane-${count.index + 1}"
      Role                                        = "control-plane"
      "kubernetes.io/cluster/${var.cluster_name}" = "owned"
    },
    var.tags
  )

  lifecycle {
    ignore_changes = [ami]
  }
}

# =============================================================================
# Attach etcd EBS Volumes
# =============================================================================

# Attach etcd volume to first control plane node
resource "aws_volume_attachment" "etcd_first" {
  count = var.control_plane_count > 0 ? 1 : 0

  device_name = "/dev/sdf"
  volume_id   = var.etcd_volume_ids[0]
  instance_id = aws_instance.control_plane_first[0].id

  force_detach = true
}

# Attach etcd volumes to additional control plane nodes
resource "aws_volume_attachment" "etcd" {
  count = var.control_plane_count > 1 ? var.control_plane_count - 1 : 0

  device_name = "/dev/sdf"
  volume_id   = var.etcd_volume_ids[count.index + 1]
  instance_id = aws_instance.control_plane[count.index].id

  force_detach = true
}

# =============================================================================
# Elastic IPs for Control Plane (optional, for static IPs)
# =============================================================================

# Uncomment if you want static public IPs for control plane nodes
# resource "aws_eip" "control_plane" {
#   count = var.control_plane_count
#
#   instance = aws_instance.control_plane[count.index].id
#   domain   = "vpc"
#
#   tags = merge(
#     {
#       Name = "${var.cluster_name}-control-plane-${count.index}-eip"
#     },
#     var.tags
#   )
# }
