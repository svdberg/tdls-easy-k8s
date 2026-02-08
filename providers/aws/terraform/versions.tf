terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
  }

  # State backend configuration
  # For initial deployments, state is stored locally
  # For production, configure S3 backend:
  #
  # backend "s3" {
  #   bucket         = "tdls-k8s-state-${account_id}"
  #   key            = "clusters/${cluster_name}/terraform.tfstate"
  #   region         = "us-east-1"
  #   encrypt        = true
  #   dynamodb_table = "tdls-k8s-state-lock"
  # }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      ManagedBy   = "tdls-easy-k8s"
      ClusterName = var.cluster_name
      Environment = var.environment
      Project     = "tdls-easy-k8s"
    }
  }
}
