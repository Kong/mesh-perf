output "cluster_endpoint" {
  description = "The URL endpoint of the EKS control plane for API communication"
  value       = module.eks.cluster_endpoint
}

output "cluster_security_group_id" {
  description = "The security group ID attached to the EKS control plane"
  value       = module.eks.cluster_security_group_id
}

output "cluster_name" {
  description = "The name of the Kubernetes EKS cluster"
  value       = var.cluster_name
}

output "ebs_csi_irsa_role" {
  description = "ARN of the IAM role used by the EBS CSI driver (IRSA) and to access ECR"
  value       = module.ebs_csi_irsa_role.iam_role_arn
}

output "region" {
  description = "AWS region where the EKS cluster is deployed"
  value       = var.region
}

output "registry" {
  description = "ECR registry for storing and retrieving container images"
  value = format("%s.dkr.ecr.%s.amazonaws.com", values(module.ecr)[0].repository_registry_id, var.region)
}
