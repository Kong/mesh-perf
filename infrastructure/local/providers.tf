terraform {
  required_providers {
    docker = {
      source = "kreuzwerker/docker"
    }
    k3d = {
      source = "pvotal-tech/k3d"
      version = "0.0.6"
    }
  }
  required_version = ">= 1.5.0"
}