# =============================================================================
# Network Load Balancer
# =============================================================================

resource "aws_lb" "nlb" {
  name               = "${var.cluster_name}-nlb"
  internal           = var.nlb_internal
  load_balancer_type = "network"
  subnets            = var.subnet_ids

  enable_cross_zone_load_balancing = true
  enable_deletion_protection       = false

  tags = merge(
    {
      Name = "${var.cluster_name}-nlb"
    },
    var.tags
  )
}

# =============================================================================
# Target Group for Kubernetes API Server
# =============================================================================

resource "aws_lb_target_group" "api_server" {
  name     = "${var.cluster_name}-api-server-tg"
  port     = 6443
  protocol = "TCP"
  vpc_id   = var.vpc_id

  health_check {
    enabled             = true
    interval            = 30
    port                = 6443
    protocol            = "TCP"
    healthy_threshold   = 3
    unhealthy_threshold = 3
  }

  deregistration_delay = 30

  tags = merge(
    {
      Name = "${var.cluster_name}-api-server-tg"
    },
    var.tags
  )
}

# =============================================================================
# Target Group Attachments
# =============================================================================

resource "aws_lb_target_group_attachment" "api_server" {
  count = length(var.control_plane_instance_ids)

  target_group_arn = aws_lb_target_group.api_server.arn
  target_id        = var.control_plane_instance_ids[count.index]
  port             = 6443
}

# =============================================================================
# Listener for Kubernetes API Server
# =============================================================================

resource "aws_lb_listener" "api_server" {
  load_balancer_arn = aws_lb.nlb.arn
  port              = 6443
  protocol          = "TCP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.api_server.arn
  }

  tags = merge(
    {
      Name = "${var.cluster_name}-api-listener"
    },
    var.tags
  )
}

# =============================================================================
# Target Group for RKE2 Registration (port 9345)
# =============================================================================

resource "aws_lb_target_group" "rke2_register" {
  name     = "${var.cluster_name}-rke2-register-tg"
  port     = 9345
  protocol = "TCP"
  vpc_id   = var.vpc_id

  health_check {
    enabled             = true
    interval            = 30
    port                = 9345
    protocol            = "TCP"
    healthy_threshold   = 3
    unhealthy_threshold = 3
  }

  deregistration_delay = 30

  tags = merge(
    {
      Name = "${var.cluster_name}-rke2-register-tg"
    },
    var.tags
  )
}

resource "aws_lb_target_group_attachment" "rke2_register" {
  count = length(var.control_plane_instance_ids)

  target_group_arn = aws_lb_target_group.rke2_register.arn
  target_id        = var.control_plane_instance_ids[count.index]
  port             = 9345
}

resource "aws_lb_listener" "rke2_register" {
  load_balancer_arn = aws_lb.nlb.arn
  port              = 9345
  protocol          = "TCP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.rke2_register.arn
  }

  tags = merge(
    {
      Name = "${var.cluster_name}-rke2-register-listener"
    },
    var.tags
  )
}
