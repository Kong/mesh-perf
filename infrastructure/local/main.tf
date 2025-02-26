provider "k3d" {}

provider "docker" {}

resource "docker_network" "network" {
  name            = "mesh-perf"
  check_duplicate = "true"
  driver          = "bridge"
  options = {
    "com.docker.network.bridge.enable_ip_masquerade" = "true"
  }
  internal = false
}

resource "k3d_cluster" "mesh-perf" {
  name    = "mesh-perf"
  servers = 1
  agents  = 2

  kube_api {
    host_ip = "127.0.0.1"
    host_port = 6550
  }

  image   = "rancher/k3s:v1.27.1-k3s1"
  network = docker_network.network.name

  port {
    host_port      = 8080
    container_port = 80
    node_filters = [
      "loadbalancer",
    ]
  }

  k3d {
    disable_load_balancer = false
    disable_image_volume  = false
  }

  kubeconfig {
    update_default_kubeconfig = true
    switch_current_context    = true
  }
  
  k3s {
    extra_args {
      arg          = "--disable=traefik"
      node_filters = ["server:0"]
    }
    extra_args {
      arg = "--disable=metrics-server"
      node_filters = ["server:0"]
    }
  }
}

resource "null_resource" "cleanup_kubeconfig" {
  provisioner "local-exec" {
    when    = destroy
    command = <<EOT
      kubectx -d "k3d-mesh-perf" || true
      kubectl config delete-context "k3d-mesh-perf" || true
      kubectl config delete-cluster "k3d-mesh-perf" || true
      kubectl config delete-user "k3d-mesh-perf" || true
    EOT
  }

  depends_on = [
    k3d_cluster.mesh-perf,
  ]
}
