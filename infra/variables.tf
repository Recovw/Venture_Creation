variable "subscription_id" {
  description = "ID subscription Azure (az account show --query id -o tsv)"
  type        = string
}

variable "location" {
  description = "Region Azure"
  type        = string
  default     = "southeastasia"
}

variable "project" {
  description = "Awalan nama semua resource"
  type        = string
  default     = "kantin"
}

variable "admin_email" {
  description = "Email akun admin awal aplikasi (dibaca ADMIN_EMAIL)"
  type        = string
}

variable "admin_password" {
  description = "Password akun admin awal aplikasi (dibaca ADMIN_PASSWORD)"
  type        = string
  sensitive   = true
}

variable "image_tag" {
  description = "Tag image untuk deploy pertama. Deploy berikutnya diurus GitHub Actions."
  type        = string
  default     = "v1"
}

variable "xendit_secret_key" {
  description = "Kosong = mode mock (tanpa Xendit asli)"
  type        = string
  default     = ""
  sensitive   = true
}

variable "xendit_callback_token" {
  description = "Token callback webhook Xendit"
  type        = string
  default     = ""
  sensitive   = true
}
