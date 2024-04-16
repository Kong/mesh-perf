terraform {
  backend "s3" {
    bucket = "mesh-perf-state"
    key = "terraform.tfstate"
    region = "us-west-1"
  }
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.45.0"
    }
  }
  required_version = ">= 1.5.0"
}