provider "aws" {
  region = var.region
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.9.0"

  name = "${var.cluster_name}-vpc"

  cidr = "10.0.0.0/16"
  azs  = var.availability_zones

  private_subnets = ["10.0.0.0/19", "10.0.32.0/19", "10.0.64.0/19"]
  public_subnets  = ["10.0.100.0/24", "10.0.101.0/24", "10.0.102.0/24"]

  enable_nat_gateway   = true
  single_nat_gateway   = true
  enable_dns_hostnames = true

  public_subnet_tags = {
    "kubernetes.io/cluster/${var.cluster_name}" = "shared"
    "kubernetes.io/role/elb"                    = 1
  }

  private_subnet_tags = {
    "kubernetes.io/cluster/${var.cluster_name}" = "shared"
    "kubernetes.io/role/internal-elb"           = 1
  }
}

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "20.17.2"

  cluster_name    = var.cluster_name
  cluster_version = "1.29"

  vpc_id                         = module.vpc.vpc_id
  subnet_ids                     = module.vpc.private_subnets
  cluster_endpoint_public_access = true

  create_cloudwatch_log_group = false
  cluster_enabled_log_types   = []

  eks_managed_node_group_defaults = {
    ami_type = "AL2_ARM_64"
  }

  eks_managed_node_groups = {
    default = {
      name = "default"

      instance_types = [var.nodes_type]

      min_size     = 1
      max_size     = var.nodes_number
      desired_size = var.nodes_number
    }

    // This node group is dedicated to observability components like Prometheus to ensure
    // they are isolated from other workloads. Prometheus resource requirements can grow
    // rapidly when deploying a large number of services, and sharing a node with other pods
    // might lead to resource shortages. By dedicating this node group, we ensure Prometheus
    // has the resources it needs to function properly without being disrupted by other workloads.
    observability = {
      name = "observability"

      instance_types = [var.nodes_type]

      min_size     = 1
      max_size     = 1
      desired_size = 1

      labels = {
        NodeGroup = "observability"
      }

      taints = {
        observability = {
          key    = "ObservabilityOnly"
          value  = "true"
          effect = "NO_SCHEDULE"
        }
      }
    }
  }

  node_security_group_additional_rules = {
    ingress_allow_access_from_control_plane = {
      type                          = "ingress"
      protocol                      = "tcp"
      from_port                     = 5443
      to_port                       = 5443
      source_cluster_security_group = true
      description                   = "Allow access from control plane to webhook port of AWS load balancer controller"
    }
  }
  authentication_mode                      = "API_AND_CONFIG_MAP"
  # required to add current use as a cluster admin
  enable_cluster_creator_admin_permissions = true
}

data "aws_iam_policy" "ebs_csi_policy" {
  arn = "arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy"
}

module "irsa-ebs-csi" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-assumable-role-with-oidc"
  version = "5.40.0"

  create_role                   = true
  role_name                     = "AmazonEKSTFEBSCSIRole-${module.eks.cluster_name}"
  provider_url                  = module.eks.oidc_provider
  role_policy_arns              = [data.aws_iam_policy.ebs_csi_policy.arn]
  oidc_fully_qualified_subjects = ["system:serviceaccount:kube-system:ebs-csi-controller-sa"]
}

resource "aws_eks_addon" "ebs-csi" {
  cluster_name             = module.eks.cluster_name
  addon_name               = "aws-ebs-csi-driver"
  addon_version            = "v1.20.0-eksbuild.1"
  service_account_role_arn = module.irsa-ebs-csi.iam_role_arn
  tags                     = {
    "eks_addon" = "ebs-csi"
    "terraform" = "true"
  }
  depends_on = [module.eks.eks_managed_node_groups]
}

resource "helm_release" "metrics_server" {
  name       = "metrics-server"
  namespace  = "kube-system"
  repository = "https://kubernetes-sigs.github.io/metrics-server/"
  chart      = "metrics-server"
  version    = "3.12.2"
  wait       = false

  values = [
    <<-YAML
    metrics:
      enabled: true
    nodeSelector:
      NodeGroup: observability
    tolerations:
    - key: ObservabilityOnly
      operator: Exists
      effect: NoSchedule
    YAML
  ]
}
