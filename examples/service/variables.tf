variable "null_application_id" {
  description = "Unique ID for the application"
  type        = number
  default     = "1255165411"
}

variable "api_key" {
  description = "API Key for consume Open Weather services"
  default     = "123456"
}

variable "specification_id" {
  description = "Specification ID for the service to be imported"
  type        = string
}
