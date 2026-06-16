# KeyOps - Infrastructure Management Platform

**English** (default) | [中文](README.zh.md)

---

**Screenshots**
![Personnel Management (English)](image/readme-en.png)

**Enterprise-grade DevOps platform built with Go** — bastion host, K8s multi-cluster, monitoring & alerts, DMS, cloud FinOps, AI assistant, and more.

## Core Features

### Feature Overview

| Category | Feature | Description | Status |
|---------|---------|-------------|--------|
| **🛡️ Bastion Host** | 🔐 SSH Gateway | Standard SSH protocol direct connection, supports traditional SSH clients | ✅ |
| | 🌐 Web Terminal | WebSocket real-time terminal, no client installation required, supports multi-session management | ✅ |
| | 🖥️ RDP Graphical | Windows remote desktop connection (via Guacamole), supports GUI operations | ✅ |
| | 🎥 Session Recording | Complete session recording and playback, supports Asciinema format | ✅ |
| | 📝 Command History | Complete command execution history and query | ✅ |
| | 📁 File Transfer | File upload/download management, supports SFTP protocol | ✅ |
| | 🚨 Command Interception | Real-time detection of dangerous commands, advanced blacklist with fuzzy/prefix/exact matching, Feishu/DingTalk alerts | ✅ |
| | 👤 System User Management | Unified management of system users (jump users) and SSH key distribution | ✅ |
| | 🔌 Proxy Agent | Edge proxy agent for connectivity across network segments, real-time session/command reporting | ✅ |
| **🔐 Authentication** | 👤 Password Login | Standard username/password authentication | ✅ |
| | 🔑 SSH Key Auth | SSH public key authentication for bastion access | ✅ |
| | 🔢 Two-Factor (TOTP) | Time-based one-time password, supports backup codes | ✅ |
| | 🔗 SSO Integration | Enterprise SSO: OIDC, Feishu (Lark), DingTalk, WeCom (WeChat Work) | ✅ |
| | 📇 LDAP/AD | LDAP directory service authentication | ✅ |
| | 🔄 Dual-Token Auth | Short-lived access token (15min JWT) + long-lived refresh token (7d, HttpOnly cookie, rotation, DB whitelist) | ✅ |
| | 🗝️ API Key | Programmatic API key authentication with role binding, supports MCP access | ✅ |
| | 🔐 Auth Override | Emergency AUTH_METHOD override for recovery scenarios | ✅ |
| **🤖 AI Assistant** | 🤖 Smart Chat | Natural-language ops assistant with Prometheus/Grafana/K8s tools, multi-turn dialogue and context | ✅ |
| | 📋 Session Management | Session list, history, multi-session switching, context persistence | ✅ |
| | ⏰ Scheduled Tasks | Scheduled expert dialogue, inspection reports, cron-based scheduling | ✅ |
| | 🛠️ Tool Sets | Built-in tools: PromQL query, Grafana visualization, K8s resource operations, analysis tools | ✅ |
| | 🧠 Multi-Model | Supports OpenAI-compatible LLM APIs, model configuration via DB | ✅ |
| **☸️ K8s Multi-Cluster** | 🌐 Cluster Management | Unified multi-cluster management, supports Token/Kubeconfig authentication | ✅ |
| | 🔐 Cluster Permissions | User/role-based cluster RBAC, supports namespace isolation, K8s-level permission rules | ✅ |
| | 📦 Workloads | Management of Deployment, DaemonSet, StatefulSet, Pod, CronJob, HPA | ✅ |
| | ⚙️ Config Management | Unified management and editing of ConfigMap and Secret | ✅ |
| | 🌐 Service Management | Creation and management of Service, Ingress | ✅ |
| | 💾 Storage Management | Configuration and management of PV, PVC, StorageClass | ✅ |
| | 📊 Cluster Monitoring | Cluster status overview, resource usage, events, pod metrics (CPU/Memory) | ✅ |
| | 📋 Operation Audit | Complete audit logs for K8s operations | ✅ |
| | 🔍 Global Search | Cross-cluster global resource search | ✅ |
| | 📜 YAML Management | Resource YAML create/edit/delete/dry-run | ✅ |
| | 🚢 Deployment | Application deployment management with rollback support | ✅ |
| | 💻 Pod Terminal | WebSocket-based pod terminal and log streaming | ✅ |
| **📋 Ticket & Workflow** | 📝 Ticket Creation | Supports daily tickets, deployment tickets, and other types | ✅ |
| | 📑 Form Templates | Visual form designer with field types: text, select, date, table, etc. | ✅ |
| | 📂 Form Categories | Form template classification management | ✅ |
| | 🔄 Approval Workflow | Multi-level approval, supports Feishu/DingTalk/WeChat Work/internal approval | ✅ |
| | 🔄 Workflow Engine | Custom workflow with multi-node, multi-approver configuration | ✅ |
| | ✅ Auto Authorization | Post-approval automatic permission rule application for host access | ✅ |
| | 📊 Ticket Statistics | Ticket status tracking, approval history, statistical analysis | ✅ |
| **🏢 Organization & Apps** | 👥 Department Management | Multi-level tree-structured department management | ✅ |
| | 📱 Application Management | Application registry with associated departments and personnel | ✅ |
| | 👤 Personnel Management | User info management, department association, role assignment | ✅ |
| | 🔧 Service Management | Service catalog with classification and detail configuration | ✅ |
| | 🔗 App-Deploy Binding | Application-to-deployment binding for release management | ✅ |
| | 📦 Registry Management | Container registry integration: Harbor, AWS ECR, Sonatype Nexus | ✅ |
| **🔐 Polymorphic Permissions** | 👥 User Groups (Roles) | Role-based permission management, supports role member CRUD | ✅ |
| | 🖥️ Host Groups | Host grouping for batch permission authorization | ✅ |
| | 👤 System Users | System user to permission rule association, many-to-many | ✅ |
| | ⏰ Time Restrictions | Permission rules support time range (valid-from/to) restrictions | ✅ |
| | 🎯 Priority Control | Permission rules with priority ranking, highest priority matched first | ✅ |
| | 📍 Fine-grained Permissions | Multi-dimension: host groups, specific hosts, system users combined | ✅ |
| | 🗂️ Menu & API Permissions | Role-based menu visibility and API endpoint access control via Casbin | ✅ |
| **📈 Monitoring & Alerts** | 📊 Prometheus Monitoring | Prometheus datasource integration, multi-instance support, metric queries | ✅ |
| | 📋 Alert Rules | PromQL alert rule management, table with sticky columns, horizontal scroll | ✅ |
| | 📋 Rule Groups | Rule group management, sidebar active state, add existing rules to group | ✅ |
| | 🎯 Alert Policies | Aggregation, suppression (restrain), silence strategies | ✅ |
| | 📢 Alert Notifications | Multi-channel: Feishu, DingTalk, Email, Webhook; template-based formatting | ✅ |
| | 📝 Alert Templates | Custom alert message templates with variable substitution | ✅ |
| | 📊 Alert Events | Full lifecycle: firing → acknowledged → resolved; event details and history | ✅ |
| | 🔔 Certificate Monitoring | SSL/TLS certificate expiration monitoring; domain, SSL, hosted certificate types | ✅ |
| | 👨‍💼 OnCall Management | Shift scheduling, duty calendar, auto/manual alert assignment | ✅ |
| | 📈 Alert Statistics | Alert trends, level distribution, strategy effectiveness | ✅ |
| | 🔗 Prometheus Webhook | Native Prometheus Alertmanager webhook receiver | ✅ |
| | 📊 Prometheus Metrics | Built-in `/metrics` endpoint exposing API request counters, proxy health, active sessions, sync errors | ✅ |
| **💾 Database Management** | 🗄️ Multi-DB Support | MySQL, PostgreSQL, MongoDB, Redis unified management | ✅ |
| | 🔍 Query Execution | SQL queries, MongoDB queries, Redis command execution with result formatting | ✅ |
| | 📝 Query Audit Logs | Complete query audit trail: user, time, IP, executed SQL | ✅ |
| | 🔐 Fine-grained Permissions | Casbin-based: instance → database → table → permission type | ✅ |
| | 🧪 Test Connection | Connection validation before saving instances | ✅ |
| **☁️ Cloud Billing & FinOps** | 💳 Cloud Accounts | Multi-cloud account credential management: AWS, Azure, GCP, Aliyun, Tencent | ✅ |
| | 📊 Cost Dashboard | Multi-cloud cost overview, trends, and comparisons | ✅ |
| | 📈 Cost Breakdown | By tag, account, region, service, resource | ✅ |
| | 📉 Optimization | Cost optimization recommendations based on usage analysis | ✅ |
| | 📋 Resource Breakdown | Resource count and expense distribution analysis | ✅ |
| | 🔄 Bill Sync | Scheduled auto-sync of cloud bills with configurable frequency | ✅ |
| **🔧 CMDB MCP** | 🖥️ CMDB Tools | CMDB (host/asset) query tools via Model Context Protocol | ✅ |
| | 🛠️ K8s Tools | K8s resource operation tools via MCP | ✅ |
| | 💰 Bill Tools | Cloud billing query tools (cost breakdown, trends, recommendations) via MCP | ✅ |
| | 🔌 MCP Service | Standard MCP server for AI tool invocation, supports API key auth | ✅ |
| **📋 Audit** | 📝 Operation Logs | Full API operation audit trail with user, action, resource, timestamp | ✅ |
| | 🗃️ Pod Command Audit | Bastion pod command recording and audit | ✅ |
| | 🗑️ Log Management | Batch deletion and retention policies | ✅ |
| **🔧 Infrastructure** | 🌐 High Availability | Multi-instance deployment, Redis distributed locks, config sync | ✅ |
| | 📊 Asset Synchronization | Auto-sync assets from Prometheus, scheduled host info updates | ✅ |
| | 🔍 Host Monitoring | Real-time host online status, health checks, connectivity probing | ✅ |
| | 🚀 Proxy Registry | Dynamic proxy registration, heartbeat, and health monitoring | ✅ |
| | 🔔 Notification Center | Centralized notification: Feishu, DingTalk, WeChat Work | ✅ |
| | 🚦 Circuit Breaker | Proxy auto-offline on failure, redundant routing | ✅ |
 ## Quick Deployment

### Requirements

- Docker 20.10+
- Docker Compose 2.0+

### MySQL Deployment (Recommended)

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

**Access System**: http://localhost:8080  
**Default Account**: `admin` / `admin123`

### PostgreSQL Deployment

KeyOps supports PostgreSQL as the backend database. Set the following environment variables in `.env` and ensure you have a PostgreSQL instance available:

```bash
DB_DRIVER=postgres
DB_HOST=127.0.0.1
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=keyops
```

## Port Description

- `8080`: HTTP (Web + API)
- `2222`: SSH Gateway
- `3306`: MySQL (optional)
- `5432`: PostgreSQL (optional)
- `6379`: Redis (optional)
- `27017`: MongoDB (optional)
- `4822`: Guacamole daemon (RDP)

## Environment Variables Configuration

Copy `.env.example` to `.env` and modify as needed (optional):

```bash
cp .env.example .env
```

Example content:

```bash
# Database configuration
MYSQL_ROOT_PASSWORD=123456
MYSQL_DATABASE=keyops
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=keyops

# Redis configuration
REDIS_ENABLED=true
REDIS_PASSWORD=

# MongoDB configuration
MONGO_INITDB_ROOT_USERNAME=admin
MONGO_INITDB_ROOT_PASSWORD=123456
MONGO_BASTION_URI=mongodb://admin:123456@mongodb:27017/keyops_bastion?authSource=admin

# Auth override (emergency)
# AUTH_METHOD=local
# ADMIN_WHITELIST=admin@example.com
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
