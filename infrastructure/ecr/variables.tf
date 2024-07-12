variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-west-1"
}

variable "cluster_name" {
  type = string
  default = "mesh-perf"
}
