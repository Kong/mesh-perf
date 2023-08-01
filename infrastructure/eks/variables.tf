variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-west-1"
}

variable "cluster_name" {
  type = string
  default = "mesh-perf"
}

variable "nodes_number" {
  type = number
}

variable "nodes_type" {
  type = string
  default = "t4g.2xlarge"
}

variable "availability_zones" {
  type = list(string)
  default = ["us-west-1b", "us-west-1c"]
}
