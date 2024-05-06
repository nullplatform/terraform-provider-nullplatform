variable "null_application_id" {
  description = "Unique ID for the application"
  type        = number
}

variable "environment" {
  description = "Environment name where the Scopes are deployed"
  default     = "dev"
}
