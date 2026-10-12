resource "random_string" "suffix" {
  length  = 6
  special = false
  upper   = false
}

resource "random_password" "db" {
  length      = 24
  special     = false
  min_upper   = 2
  min_lower   = 2
  min_numeric = 2
}

resource "random_password" "jwt" {
  length  = 48
  special = false
}

# Resource group
resource "azurerm_resource_group" "rg" {
  name     = "rg-${var.project}"
  location = var.location
}

# Container Registry (gudang image) 
resource "azurerm_container_registry" "acr" {
  name                = "${var.project}acr${random_string.suffix.result}"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  sku                 = "Basic"
  admin_enabled       = true
}

# Database PostgreSQL 
resource "azurerm_postgresql_flexible_server" "db" {
  name                   = "${var.project}-db-${random_string.suffix.result}"
  resource_group_name    = azurerm_resource_group.rg.name
  location               = azurerm_resource_group.rg.location
  version                = "16"
  administrator_login    = "kantinadmin"
  administrator_password = random_password.db.result
  sku_name               = "B_Standard_B1ms"
  storage_mb             = 32768
  backup_retention_days  = 7

  lifecycle {
    ignore_changes = [zone]
  }
}

resource "azurerm_postgresql_flexible_server_database" "kantin" {
  name      = "kantin"
  server_id = azurerm_postgresql_flexible_server.db.id
  charset   = "UTF8"
  collation = "en_US.utf8"
}

# 0.0.0.0 = hanya layanan Azure (termasuk Container Apps) yang boleh konek
resource "azurerm_postgresql_flexible_server_firewall_rule" "azure_services" {
  name             = "allow-azure-services"
  server_id        = azurerm_postgresql_flexible_server.db.id
  start_ip_address = "0.0.0.0"
  end_ip_address   = "0.0.0.0"
}

locals {
  db_dsn = "host=${azurerm_postgresql_flexible_server.db.fqdn} user=kantinadmin password=${random_password.db.result} dbname=${azurerm_postgresql_flexible_server_database.kantin.name} port=5432 sslmode=require TimeZone=Asia/Jakarta"
}

# Log + environment Container Apps 
resource "azurerm_log_analytics_workspace" "logs" {
  name                = "${var.project}-logs"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  sku                 = "PerGB2018"
  retention_in_days   = 30
}

resource "azurerm_container_app_environment" "env" {
  name                       = "${var.project}-env"
  resource_group_name        = azurerm_resource_group.rg.name
  location                   = azurerm_resource_group.rg.location
  log_analytics_workspace_id = azurerm_log_analytics_workspace.logs.id
}

# Aplikasi backend Go
resource "azurerm_container_app" "api" {
  name                         = "${var.project}-api"
  resource_group_name          = azurerm_resource_group.rg.name
  container_app_environment_id = azurerm_container_app_environment.env.id
  revision_mode                = "Single"

  registry {
    server               = azurerm_container_registry.acr.login_server
    username             = azurerm_container_registry.acr.admin_username
    password_secret_name = "acr-password"
  }

  secret {
    name  = "acr-password"
    value = azurerm_container_registry.acr.admin_password
  }

  secret {
    name  = "db-dsn"
    value = local.db_dsn
  }

  secret {
    name  = "jwt-secret"
    value = random_password.jwt.result
  }

  secret {
    name  = "admin-password"
    value = var.admin_password
  }

  dynamic "secret" {
    for_each = var.xendit_secret_key != "" ? [1] : []
    content {
      name  = "xendit-key"
      value = var.xendit_secret_key
    }
  }

  dynamic "secret" {
    for_each = var.xendit_callback_token != "" ? [1] : []
    content {
      name  = "xendit-token"
      value = var.xendit_callback_token
    }
  }

  ingress {
    external_enabled = true
    target_port      = 8080

    traffic_weight {
      latest_revision = true
      percentage      = 100
    }
  }

  template {
    min_replicas = 1
    max_replicas = 1

    container {
      name   = "api"
      image  = "${azurerm_container_registry.acr.login_server}/kantin-api:${var.image_tag}"
      cpu    = 0.25
      memory = "0.5Gi"

      env {
        name        = "DB_DSN"
        secret_name = "db-dsn"
      }
      env {
        name        = "JWT_SECRET"
        secret_name = "jwt-secret"
      }
      env {
        name  = "ADMIN_EMAIL"
        value = var.admin_email
      }
      env {
        name        = "ADMIN_PASSWORD"
        secret_name = "admin-password"
      }
      env {
        name  = "GIN_MODE"
        value = "release"
      }
      env {
        name  = "PORT"
        value = "8080"
      }

      dynamic "env" {
        for_each = var.xendit_secret_key != "" ? [1] : []
        content {
          name        = "XENDIT_SECRET_KEY"
          secret_name = "xendit-key"
        }
      }
      dynamic "env" {
        for_each = var.xendit_callback_token != "" ? [1] : []
        content {
          name        = "XENDIT_CALLBACK_TOKEN"
          secret_name = "xendit-token"
        }
      }
    }
  }

  # Image versi baru di-deploy oleh GitHub Actions (az containerapp update),
  # jadi Terraform tidak boleh mengembalikannya ke image_tag awal.
  lifecycle {
    ignore_changes = [template[0].container[0].image]
  }
}