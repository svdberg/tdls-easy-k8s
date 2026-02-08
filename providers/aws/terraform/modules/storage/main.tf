# =============================================================================
# EBS Volumes for etcd
# =============================================================================

resource "aws_ebs_volume" "etcd" {
  count = var.control_plane_count

  availability_zone = var.availability_zones[count.index % length(var.availability_zones)]
  size              = var.etcd_volume_size
  type              = var.etcd_volume_type
  iops              = var.etcd_volume_type == "gp3" || var.etcd_volume_type == "io1" || var.etcd_volume_type == "io2" ? var.etcd_volume_iops : null
  throughput        = var.etcd_volume_type == "gp3" ? var.etcd_volume_throughput : null
  encrypted         = var.kms_key_id != null
  kms_key_id        = var.kms_key_id

  tags = merge(
    {
      Name                                        = "${var.cluster_name}-etcd-${count.index}"
      Component                                   = "etcd"
      "kubernetes.io/cluster/${var.cluster_name}" = "owned"
    },
    var.tags
  )
}
