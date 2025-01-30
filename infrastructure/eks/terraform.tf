terraform {
  backend "s3" {
    bucket = "mesh-perf-state"
    key    = "terraform.tfstate"
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
