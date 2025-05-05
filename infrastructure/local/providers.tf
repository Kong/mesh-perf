terraform {
  required_providers {
    docker = {
      source = "kreuzwerker/docker"
      version = "3.4.0"
    }
    k3d = {
      source = "pvotal-tech/k3d"
      version = "0.0.7"
    }
  }
  required_version = ">= 1.5.0"
}
