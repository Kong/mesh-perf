provider "aws" {
  region = var.region
}

data "aws_iam_role" "eks_csi_role" {
  name = "AmazonEKSTFEBSCSIRole-${var.cluster_name}"
}

module "ecr" {
  source = "terraform-aws-modules/ecr/aws"
  version = "2.2.0"
  for_each = toset(["kuma-dp", "fake-service"])

  repository_name = each.key
  repository_force_delete = "true"

  repository_read_write_access_arns = [data.aws_iam_role.eks_csi_role.arn]
  repository_lifecycle_policy       = jsonencode({
    rules = [
      {
        rulePriority = 1,
        description  = "Keep last 10 images",
        selection    = {
          tagStatus     = "any",
          countType     = "imageCountMoreThan",
          countNumber   = 10
        },
        action = {
          type = "expire"
        }
      }
    ]
  })
}
