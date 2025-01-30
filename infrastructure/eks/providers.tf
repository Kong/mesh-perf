terraform {
  backend "s3" {
    bucket = "mesh-perf-state"
    key = "terraform.tfstate"
    region = "us-west-1"
  }
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.57.0"
    }

    helm = {
      source  = "hashicorp/helm"
      version = "2.17.0"
    }
  }
  required_version = ">= 1.5.0"
}

provider "helm" {
  kubernetes {
    host = module.eks.cluster_endpoint
    cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)

    exec {
      api_version = "client.authentication.k8s.io/v1beta1"
      command = "aws"
      # This requires the awscli to be installed locally where Terraform is executed
      args = ["eks", "get-token", "--cluster-name", module.eks.cluster_name]
    }
  }
}
