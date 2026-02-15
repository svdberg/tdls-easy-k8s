# =============================================================================
# KMS Keys for Encryption
# =============================================================================

resource "aws_kms_key" "cluster" {
  count = var.enable_encryption ? 1 : 0

  description             = "KMS key for ${var.cluster_name} cluster encryption"
  deletion_window_in_days = var.kms_key_deletion_window
  enable_key_rotation     = true

  tags = merge(
    {
      Name = "${var.cluster_name}-kms-key"
    },
    var.tags
  )
}

resource "aws_kms_alias" "cluster" {
  count = var.enable_encryption ? 1 : 0

  name          = "alias/${var.cluster_name}-cluster"
  target_key_id = aws_kms_key.cluster[0].key_id
}

# =============================================================================
# Control Plane IAM Role
# =============================================================================

resource "aws_iam_role" "control_plane" {
  name_prefix = "${var.cluster_name}-control-plane-"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
      }
    ]
  })

  tags = merge(
    {
      Name = "${var.cluster_name}-control-plane-role"
    },
    var.tags
  )
}

resource "aws_iam_instance_profile" "control_plane" {
  name_prefix = "${var.cluster_name}-control-plane-"
  role        = aws_iam_role.control_plane.name

  tags = merge(
    {
      Name = "${var.cluster_name}-control-plane-profile"
    },
    var.tags
  )
}

# ECR Access Policy
resource "aws_iam_role_policy" "control_plane_ecr" {
  name_prefix = "ecr-access-"
  role        = aws_iam_role.control_plane.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ecr:GetAuthorizationToken",
          "ecr:BatchCheckLayerAvailability",
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchGetImage"
        ]
        Resource = "*"
      }
    ]
  })
}

# EBS Management Policy
resource "aws_iam_role_policy" "control_plane_ebs" {
  name_prefix = "ebs-access-"
  role        = aws_iam_role.control_plane.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ec2:AttachVolume",
          "ec2:DetachVolume",
          "ec2:DescribeVolumes",
          "ec2:DescribeVolumeStatus",
          "ec2:CreateVolume",
          "ec2:DeleteVolume",
          "ec2:CreateSnapshot",
          "ec2:DeleteSnapshot",
          "ec2:DescribeSnapshots",
          "ec2:CreateTags"
        ]
        Resource = "*"
      }
    ]
  })
}

# S3 Access Policy (for state and backups)
resource "aws_iam_role_policy" "control_plane_s3" {
  name_prefix = "s3-access-"
  role        = aws_iam_role.control_plane.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject",
          "s3:ListBucket"
        ]
        Resource = [
          "arn:aws:s3:::${var.state_bucket}",
          "arn:aws:s3:::${var.state_bucket}/*"
        ]
      }
    ]
  })
}

# KMS Access Policy
resource "aws_iam_role_policy" "control_plane_kms" {
  count = var.enable_encryption ? 1 : 0

  name_prefix = "kms-access-"
  role        = aws_iam_role.control_plane.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "kms:Decrypt",
          "kms:Encrypt",
          "kms:GenerateDataKey",
          "kms:DescribeKey"
        ]
        Resource = aws_kms_key.cluster[0].arn
      }
    ]
  })
}

# CloudWatch Logs Policy
resource "aws_iam_role_policy" "control_plane_cloudwatch" {
  count = var.enable_cloudwatch_logs ? 1 : 0

  name_prefix = "cloudwatch-logs-"
  role        = aws_iam_role.control_plane.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents",
          "logs:DescribeLogStreams"
        ]
        Resource = "arn:aws:logs:*:*:log-group:/aws/rke2/${var.cluster_name}/*"
      }
    ]
  })
}

# Systems Manager Session Manager Policy
resource "aws_iam_role_policy_attachment" "control_plane_ssm" {
  count = var.enable_session_manager ? 1 : 0

  role       = aws_iam_role.control_plane.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

# =============================================================================
# Worker IAM Role
# =============================================================================

resource "aws_iam_role" "worker" {
  name_prefix = "${var.cluster_name}-worker-"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
      }
    ]
  })

  tags = merge(
    {
      Name = "${var.cluster_name}-worker-role"
    },
    var.tags
  )
}

resource "aws_iam_instance_profile" "worker" {
  name_prefix = "${var.cluster_name}-worker-"
  role        = aws_iam_role.worker.name

  tags = merge(
    {
      Name = "${var.cluster_name}-worker-profile"
    },
    var.tags
  )
}

# ECR Access Policy
resource "aws_iam_role_policy" "worker_ecr" {
  name_prefix = "ecr-access-"
  role        = aws_iam_role.worker.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ecr:GetAuthorizationToken",
          "ecr:BatchCheckLayerAvailability",
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchGetImage"
        ]
        Resource = "*"
      }
    ]
  })
}

# EBS Management Policy (for persistent volumes)
resource "aws_iam_role_policy" "worker_ebs" {
  name_prefix = "ebs-access-"
  role        = aws_iam_role.worker.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ec2:AttachVolume",
          "ec2:DetachVolume",
          "ec2:DescribeVolumes",
          "ec2:DescribeVolumeStatus",
          "ec2:CreateVolume",
          "ec2:DeleteVolume",
          "ec2:CreateSnapshot",
          "ec2:DeleteSnapshot",
          "ec2:DescribeSnapshots",
          "ec2:CreateTags"
        ]
        Resource = "*"
        Condition = {
          StringEquals = {
            "ec2:ResourceTag/kubernetes.io/cluster/${var.cluster_name}" = "owned"
          }
        }
      }
    ]
  })
}

# S3 Access Policy (limited to cluster bucket)
resource "aws_iam_role_policy" "worker_s3" {
  name_prefix = "s3-access-"
  role        = aws_iam_role.worker.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:ListBucket"
        ]
        Resource = [
          "arn:aws:s3:::${var.state_bucket}",
          "arn:aws:s3:::${var.state_bucket}/*"
        ]
      }
    ]
  })
}

# CloudWatch Logs Policy
resource "aws_iam_role_policy" "worker_cloudwatch" {
  count = var.enable_cloudwatch_logs ? 1 : 0

  name_prefix = "cloudwatch-logs-"
  role        = aws_iam_role.worker.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents",
          "logs:DescribeLogStreams"
        ]
        Resource = "arn:aws:logs:*:*:log-group:/aws/rke2/${var.cluster_name}/*"
      }
    ]
  })
}

# Systems Manager Session Manager Policy
resource "aws_iam_role_policy_attachment" "worker_ssm" {
  count = var.enable_session_manager ? 1 : 0

  role       = aws_iam_role.worker.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

# AWS Secrets Manager Policy (for External Secrets Operator)
resource "aws_iam_role_policy" "worker_secrets_manager" {
  count = var.enable_secrets_manager ? 1 : 0

  name_prefix = "secrets-manager-"
  role        = aws_iam_role.worker.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue",
          "secretsmanager:ListSecrets",
          "secretsmanager:DescribeSecret"
        ]
        Resource = "*"
      }
    ]
  })
}
