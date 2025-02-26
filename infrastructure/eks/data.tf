data "aws_eks_cluster" "eks" {
  name = module.eks.cluster_name
  depends_on = [
    module.eks
  ]
}

data "aws_eks_cluster_auth" "eks" {
  name = module.eks.cluster_name
  depends_on = [
    data.aws_eks_cluster.eks
  ]
}

data "aws_caller_identity" "current" {}
