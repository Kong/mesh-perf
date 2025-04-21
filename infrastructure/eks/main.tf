data "aws_caller_identity" "current" {}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.19.0"

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
  version = "20.36.0"

  cluster_name    = var.cluster_name
  cluster_version = var.cluster_version

  vpc_id                         = module.vpc.vpc_id
  subnet_ids                     = module.vpc.private_subnets
  cluster_endpoint_public_access = true

  # Enable the CloudWatch log group and detailed EKS logs (API, audit, etc.) only when `local.debug` is true.
  # This helps with troubleshooting and deeper visibility while avoiding unnecessary overhead otherwise.
  create_cloudwatch_log_group = local.debug
  cluster_enabled_log_types   = local.debug ? [
    "api",
    "audit",
    "authenticator",
    "controllerManager",
    "scheduler"
  ] : []

  authentication_mode                      = "API_AND_CONFIG_MAP"
  enable_cluster_creator_admin_permissions = true

  # On local environments, the user credentials are already configured automatically, so we don’t need to set them again.
  # This configuration is only necessary on CI to grant access to the cluster from our CI role/account.
  access_entries = local.ci ? {
    poweruser = {
      principal_arn = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:role/poweruser"

      policy_associations = {
        cluster_admin = {
          policy_arn = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"
          access_scope = {
            type       = "cluster"
            namespaces = []
          }
        }
      }
    }
  } : {}

  eks_managed_node_group_defaults = {
    ami_type = "AL2_ARM_64"
  }

  eks_managed_node_groups = {
    default = {
      name = "default"

      instance_types = [var.nodes_type]

      min_size     = var.nodes_number
      max_size     = var.nodes_number
      desired_size = var.nodes_number
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

  cluster_addons = {
    aws-ebs-csi-driver = {
      most_recent              = true
      service_account_role_arn = module.ebs_csi_irsa_role.iam_role_arn
      configuration_values = jsonencode({
        sidecars : {
          snapshotter : {
            forceEnable : false
          }
        },
        controller : {
          volumeModificationFeature : {
            enabled : true
          }
        }
      })
    }

    eks-pod-identity-agent = {
      most_recent = true
    }
  }
}

module "ecr" {
  source   = "terraform-aws-modules/ecr/aws"
  version  = "2.4.0"
  for_each = toset(["kuma-dp", "fake-service"])

  repository_name         = each.key
  repository_force_delete = "true"

  repository_read_write_access_arns = [module.ebs_csi_irsa_role.iam_role_arn]

  repository_lifecycle_policy = jsonencode({
    rules = [
      {
        rulePriority = 1,
        description  = "Keep last 10 images",
        selection = {
          tagStatus   = "any",
          countType   = "imageCountMoreThan",
          countNumber = 10
        },
        action = {
          type = "expire"
        }
      }
    ]
  })
}

module "ebs_csi_irsa_role" {
  source = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"

  role_name             = "ebs-csi-${var.cluster_name}"
  attach_ebs_csi_policy = true

  oidc_providers = {
    eks = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["kube-system:ebs-csi-controller-sa"]
    }
  }
}

# The default gp2 storage class does not include the `storageclass.kubernetes.io/is-default-class` annotation,
# which some installations (e.g., `kumactl install observability`) require.
# Rather than modifying the default gp2 class, we create a new gp3 storage class and mark it as the default.
resource "kubernetes_storage_class" "gp3" {
  metadata {
    name = "gp3"
    annotations = {
      "storageclass.kubernetes.io/is-default-class" = "true"
    }
  }

  storage_provisioner    = "ebs.csi.aws.com"
  reclaim_policy         = "Delete"
  allow_volume_expansion = true
  volume_binding_mode    = "WaitForFirstConsumer"
  parameters = {
    fsType    = "ext4"
    encrypted = false
    type      = "gp3"
  }
}

# Installs the metrics-server Helm chart, which is used primarily for debugging and detailed metrics analysis.
# It is only created when `local.debug` is true to avoid unnecessary overhead in non-debug environments.
resource "helm_release" "metrics_server" {
  count = local.debug ? 1 : 0

  name       = "metrics-server"
  chart      = "metrics-server"
  version    = "3.12.2"
  repository = "https://kubernetes-sigs.github.io/metrics-server"
  namespace  = "kube-system"
  wait       = false

  values = [
    <<-YAML
    metrics:
      enabled: true
    YAML
  ]
}

# Updates the local kubeconfig with the new EKS cluster details, allowing kubectl to interact
# with the cluster.
resource "null_resource" "update_kubeconfig" {
  provisioner "local-exec" {
    command = "aws eks update-kubeconfig --region ${var.region} --name ${var.cluster_name}"
  }

  depends_on = [
    module.eks
  ]
}

# This resource cleans up local kubeconfig entries for the EKS cluster when Terraform destroys the infrastructure.
# It is only created if `local.ci` is false (to avoid interfering with CI environments). On destroy, it removes
# the related context, cluster, and user from the local kubeconfig, ensuring there are no lingering references.
resource "null_resource" "cleanup_kubeconfig" {
  count = local.ci ? 0 : 1

  triggers = {
    context_name = "arn:aws:eks:${var.region}:${data.aws_caller_identity.current.account_id}:cluster/${var.cluster_name}"
  }

  provisioner "local-exec" {
    when    = destroy
    command = <<EOT
      kubectx -d "${self.triggers.context_name}" || true
      kubectl config delete-context "${self.triggers.context_name}" || true
      kubectl config delete-cluster "${self.triggers.context_name}" || true
      kubectl config delete-user "${self.triggers.context_name}" || true
    EOT
  }

  depends_on = [
    module.eks
  ]
}
