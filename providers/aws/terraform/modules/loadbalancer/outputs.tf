output "nlb_arn" {
  description = "NLB ARN"
  value       = aws_lb.nlb.arn
}

output "nlb_dns_name" {
  description = "NLB DNS name"
  value       = aws_lb.nlb.dns_name
}

output "nlb_zone_id" {
  description = "NLB Route53 zone ID"
  value       = aws_lb.nlb.zone_id
}

output "target_group_arn" {
  description = "API server target group ARN"
  value       = aws_lb_target_group.api_server.arn
}

output "ingress_nlb_dns_name" {
  description = "Ingress NLB DNS name"
  value       = var.enable_ingress ? aws_lb.ingress[0].dns_name : null
}

output "ingress_nlb_zone_id" {
  description = "Ingress NLB Route53 zone ID"
  value       = var.enable_ingress ? aws_lb.ingress[0].zone_id : null
}
