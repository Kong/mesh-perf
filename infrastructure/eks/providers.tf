data "aws_eks_cluster" "cluster" {
  name = module.eks.cluster_name
}

data "aws_eks_cluster_auth" "cluster" {
  name = module.eks.cluster_name
  depends_on = [
    data.aws_eks_cluster.cluster
  ]
}

provider "aws" {
  region = var.region
}

provider "helm" {
  kubernetes {
    host  = module.eks.cluster_endpoint
    token = data.aws_eks_cluster_auth.cluster.token
    cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)
  }
}
