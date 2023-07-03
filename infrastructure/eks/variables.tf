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
  default = 3
}

variable "nodes_type" {
  type = string
  default = "t3.medium"
}

variable "availability_zones" {
  type = list(string)
  default = ["us-west-1b", "us-west-1c"]
}