output "app_url" {
  description = "URL backend (dipakai frontend/mobile sebagai base URL, tambahkan /api)"
  value       = "https://${azurerm_container_app.api.ingress[0].fqdn}"
}

output "xendit_webhook_url" {
  description = "Isi di dashboard Xendit sebagai callback URL invoice"
  value       = "https://${azurerm_container_app.api.ingress[0].fqdn}/api/webhooks/xendit/invoice"
}

output "acr_name" {
  description = "Nama Container Registry (untuk az acr build dan GitHub variable ACR_NAME)"
  value       = azurerm_container_registry.acr.name
}

output "db_host" {
  description = "Host PostgreSQL"
  value       = azurerm_postgresql_flexible_server.db.fqdn
}
