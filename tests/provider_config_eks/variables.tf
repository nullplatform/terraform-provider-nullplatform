variable "nrn" {
  type        = string
  description = "The NRN (Nullplatform Resource Name) for the target scope."
}

variable "cluster_name" {
  type        = string
  description = "The EKS cluster name."
}

variable "cluster_namespace" {
  type        = string
  description = "The Kubernetes namespace to use in the cluster."
}

variable "private_lb_name" {
  type        = string
  description = "The private load balancer name."
}

variable "public_lb_name" {
  type        = string
  description = "The public load balancer name."
}

variable "additional_public_names" {
  type        = list(string)
  description = "Additional public load balancer names."
  default     = []
}

variable "additional_private_names" {
  type        = list(string)
  description = "Additional private load balancer names."
  default     = []
}

variable "alb_capacity_threshold" {
  type        = number
  description = "ALB capacity threshold percentage."
  default     = 75
}

variable "dimensions" {
  type        = string
  description = "A JSON-encoded string of dimensions for the provider config."
  default     = "{}"
}
