output "region" {
  description = "AWS region"
  value       = var.region
}

output "ecr_registry" {
  description = "ECR registry"
  value       = format("%s.dkr.ecr.%s.amazonaws.com", values(module.ecr)[0].repository_registry_id, var.region)
}
