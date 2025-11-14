variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-west-1"
}

variable "availability_zones" {
  description = "List of availability zones to use"
  type        = list(string)
  default     = ["us-west-1b", "us-west-1c"]
}

variable "cluster_name" {
  description = "Name of the EKS cluster"
  type        = string
  default     = "mesh-perf"
}

variable "cluster_version" {
  description = "Kubernetes version of the EKS cluster"
  type        = number
  default     = 1.32
}

variable "nodes_number" {
  description = "Number of worker nodes in the cluster"
  type        = number
  default     = 1
}

variable "nodes_type" {
  description = "EC2 instance type for the worker nodes"
  type        = string
  default     = "t4g.2xlarge"
}

variable "ci" {
  description = "Set to true if run in CI"
  type        = string
}

variable "debug" {
  description = "Set to true if run in CI with RUNNER_DEBUG enabled"
  type        = string
}
