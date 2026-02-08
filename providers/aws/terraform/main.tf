# =============================================================================
# tdls-easy-k8s - Main Terraform Configuration
# =============================================================================

locals {
  # Use provided AZs or auto-detect
  availability_zones = length(var.availability_zones) > 0 ? var.availability_zones : slice(data.aws_availability_zones.available.names, 0, 3)

  # Use provided AMI or auto-detected Ubuntu AMI
  ami_id = var.ami_id != "" ? var.ami_id : data.aws_ami.ubuntu[0].id

  # Use provided cluster token or generated one
  cluster_token = var.cluster_token != "" ? var.cluster_token : random_password.cluster_token[0].result

  # Determine API endpoint (NLB DNS or first control plane IP)
  api_endpoint = var.enable_nlb ? module.loadbalancer[0].nlb_dns_name : module.control_plane.first_node_ip

  # Common tags
  common_tags = merge(
    {
      Cluster     = var.cluster_name
      Environment = var.environment
      ManagedBy   = "tdls-easy-k8s"
    },
    var.additional_tags
  )
}

# =============================================================================
# Networking Module
# =============================================================================

module "networking" {
  source = "./modules/networking"

  cluster_name         = var.cluster_name
  vpc_cidr             = var.vpc_cidr
  public_subnet_cidrs  = var.public_subnet_cidrs
  private_subnet_cidrs = var.private_subnet_cidrs
  availability_zones   = local.availability_zones
  enable_nat_gateway   = var.enable_nat_gateway
  single_nat_gateway   = var.single_nat_gateway
  enable_vpc_endpoints = var.enable_vpc_endpoints

  tags = local.common_tags
}

# =============================================================================
# Security Module
# =============================================================================

module "security" {
  source = "./modules/security"

  cluster_name             = var.cluster_name
  vpc_id                   = module.networking.vpc_id
  vpc_cidr                 = module.networking.vpc_cidr
  api_server_allowed_cidrs = var.api_server_allowed_cidrs
  enable_nlb               = var.enable_nlb

  tags = local.common_tags

  depends_on = [module.networking]
}

# =============================================================================
# IAM Module
# =============================================================================

module "iam" {
  source = "./modules/iam"

  cluster_name            = var.cluster_name
  state_bucket            = var.state_bucket
  enable_encryption       = var.enable_encryption
  kms_key_deletion_window = var.kms_key_deletion_window
  enable_session_manager  = var.enable_session_manager
  enable_cloudwatch_logs  = var.enable_cloudwatch_logs

  tags = local.common_tags
}

# =============================================================================
# Storage Module
# =============================================================================

module "storage" {
  source = "./modules/storage"

  cluster_name           = var.cluster_name
  control_plane_count    = var.control_plane_count
  availability_zones     = local.availability_zones
  etcd_volume_size       = var.etcd_volume_size
  etcd_volume_type       = var.etcd_volume_type
  etcd_volume_iops       = var.etcd_volume_iops
  etcd_volume_throughput = var.etcd_volume_throughput
  kms_key_id             = var.enable_encryption ? module.iam.kms_key_id : null

  tags = local.common_tags

  depends_on = [module.iam]
}

# =============================================================================
# Control Plane Module
# =============================================================================

module "control_plane" {
  source = "./modules/compute/control-plane"

  cluster_name              = var.cluster_name
  control_plane_count       = var.control_plane_count
  instance_type             = var.control_plane_instance_type
  ami_id                    = local.ami_id
  subnet_ids                = module.networking.public_subnet_ids
  security_group_ids        = [module.security.control_plane_sg_id]
  iam_instance_profile_name = module.iam.control_plane_instance_profile_name
  ssh_key_name              = var.ssh_key_name
  root_volume_size          = var.control_plane_root_volume_size
  root_volume_type          = var.control_plane_root_volume_type
  etcd_volume_ids           = module.storage.etcd_volume_ids
  cluster_token             = local.cluster_token
  rke2_version              = var.rke2_version
  cni_plugin                = var.cni_plugin
  cluster_cidr              = var.cluster_cidr
  service_cidr              = var.service_cidr
  cluster_dns               = var.cluster_dns
  state_bucket              = var.state_bucket
  nlb_dns_name              = "" # Not needed during instance creation
  enable_encryption         = var.enable_encryption
  kms_key_id                = var.enable_encryption ? module.iam.kms_key_id : null

  tags = local.common_tags

  depends_on = [
    module.networking,
    module.security,
    module.iam,
    module.storage
  ]
}

# =============================================================================
# Load Balancer Module
# =============================================================================

module "loadbalancer" {
  count  = var.enable_nlb ? 1 : 0
  source = "./modules/loadbalancer"

  cluster_name               = var.cluster_name
  vpc_id                     = module.networking.vpc_id
  subnet_ids                 = module.networking.public_subnet_ids
  control_plane_instance_ids = module.control_plane.instance_ids
  nlb_internal               = var.nlb_internal

  tags = local.common_tags

  depends_on = [module.control_plane]
}

# =============================================================================
# Worker Module
# =============================================================================

module "worker" {
  source = "./modules/compute/worker"

  cluster_name              = var.cluster_name
  worker_count              = var.worker_count
  instance_type             = var.worker_instance_type
  ami_id                    = local.ami_id
  subnet_ids                = module.networking.private_subnet_ids
  security_group_ids        = [module.security.worker_sg_id]
  iam_instance_profile_name = module.iam.worker_instance_profile_name
  ssh_key_name              = var.ssh_key_name
  root_volume_size          = var.worker_root_volume_size
  root_volume_type          = var.worker_root_volume_type
  cluster_token             = local.cluster_token
  rke2_version              = var.rke2_version
  api_endpoint              = local.api_endpoint
  enable_spot_instances     = var.enable_spot_instances
  enable_encryption         = var.enable_encryption
  kms_key_id                = var.enable_encryption ? module.iam.kms_key_id : null

  tags = local.common_tags

  depends_on = [
    module.networking,
    module.security,
    module.iam,
    module.control_plane
  ]
}
