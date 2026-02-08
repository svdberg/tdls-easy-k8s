# =============================================================================
# Worker EC2 Instances
# =============================================================================

resource "aws_instance" "worker" {
  count = var.worker_count

  ami                    = var.ami_id
  instance_type          = var.instance_type
  subnet_id              = var.subnet_ids[count.index % length(var.subnet_ids)]
  vpc_security_group_ids = var.security_group_ids
  iam_instance_profile   = var.iam_instance_profile_name
  key_name               = var.ssh_key_name != "" ? var.ssh_key_name : null

  # Spot instance configuration
  instance_market_options {
    market_type = var.enable_spot_instances ? "spot" : null

    dynamic "spot_options" {
      for_each = var.enable_spot_instances ? [1] : []
      content {
        spot_instance_type             = "one-time"
        instance_interruption_behavior = "terminate"
      }
    }
  }

  root_block_device {
    volume_size           = var.root_volume_size
    volume_type           = var.root_volume_type
    encrypted             = var.enable_encryption
    kms_key_id            = var.kms_key_id
    delete_on_termination = true

    tags = merge(
      {
        Name = "${var.cluster_name}-worker-${count.index}-root"
      },
      var.tags
    )
  }

  user_data = base64encode(templatefile("${path.module}/user-data.tpl", {
    cluster_name  = var.cluster_name
    cluster_token = var.cluster_token
    rke2_version  = var.rke2_version
    api_endpoint  = var.api_endpoint
    node_index    = count.index
  }))

  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
  }

  tags = merge(
    {
      Name                                        = "${var.cluster_name}-worker-${count.index}"
      Role                                        = "worker"
      "kubernetes.io/cluster/${var.cluster_name}" = "owned"
    },
    var.tags
  )

  lifecycle {
    ignore_changes = [ami]
  }
}
