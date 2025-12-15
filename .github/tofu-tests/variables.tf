variable "organization_name" {
  description = "The name of the Infradots organization"
  type        = string
}

variable "infradots_token" {
  description = "API token for authenticating with Infradots"
  type        = string
  sensitive   = true
}

variable "infradots_hostname" {
  description = "The hostname of the Infradots Platform (optional, defaults to api.infradots.com)"
  type        = string
  default     = "api.infradots.com"
}

