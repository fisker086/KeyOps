# KeyOps - 基础设施管理平台

[English](README.md) | **中文**

---

**相关截图**
![部门管理（中文）](image/readme-zh.jpg)

**基于 Go 的企业级 DevOps 一体化平台** — 堡垒机、K8s 多集群、监控告警、数据库管理、云费用 FinOps、AI 助手等。

## 核心功能

### 功能概览表

| 功能分类 | 功能名称 | 功能描述 | 状态 |
|---------|---------|---------|------|
| **🛡️ 堡垒机** | 🔐 SSH Gateway | 标准 SSH 协议直连，支持传统 SSH 客户端工具 | ✅ |
| | 🌐 Web Terminal | WebSocket 实时终端，无需安装客户端，支持多会话管理 | ✅ |
| | 🖥️ RDP 图形化 | Windows 远程桌面连接（基于 Guacamole），支持图形界面操作 | ✅ |
| | 🎥 会话录制 | 完整的会话录制和回放功能，支持 Asciinema 格式 | ✅ |
| | 📝 命令历史 | 完整的命令执行历史记录和查询 | ✅ |
| | 📁 文件传输 | 文件上传/下载管理，支持 SFTP 协议 | ✅ |
| | 🚨 命令拦截 | 实时检测危险命令，支持模糊/前缀/精确匹配黑名单，飞书/钉钉告警 | ✅ |
| | 👤 系统用户管理 | 系统用户（跳板用户）的统一管理和 SSH 密钥分发 | ✅ |
| | 🔌 代理 Agent | 边缘网络代理，跨网段连接，实时会话/命令上报 | ✅ |
| **🔐 认证体系** | 👤 密码登录 | 标准用户名/密码认证 | ✅ |
| | 🔑 SSH 密钥 | SSH 公钥认证，用于堡垒机登录 | ✅ |
| | 🔢 双因子 TOTP | 基于时间的一次性密码，支持备份码 | ✅ |
| | 🔗 单点登录 SSO | 企业级 SSO：OIDC、飞书、钉钉、企业微信 | ✅ |
| | 📇 LDAP/AD | LDAP 目录服务认证集成 | ✅ |
| | 🔄 双 Token 认证 | 短生命周期 access token（15分 JWT）+ 长生命周期 refresh token（7天 HttpOnly Cookie、轮换、DB 白名单） | ✅ |
| | 🗝️ API 密钥 | 程序化 API 密钥认证，支持角色绑定，可用于 MCP 访问 | ✅ |
| | 🔐 认证覆盖 | 紧急 AUTH_METHOD 覆盖，用于故障恢复场景 | ✅ |
| **🤖 AI 助手** | 🤖 智能对话 | 自然语言运维助手，集成 Prometheus/Grafana/K8s 工具集，多轮对话 | ✅ |
| | 📋 会话管理 | 会话列表、历史记录、多会话切换，上下文持久化 | ✅ |
| | ⏰ 定时任务 | 定时触发专家对话与巡检报告，支持 Cron 调度 | ✅ |
| | 🛠️ 工具集 | 内置 PromQL 查询、Grafana 可视化、K8s 资源操作、分析工具 | ✅ |
| | 🧠 多模型 | 支持 OpenAI 兼容 LLM API，模型配置数据库管理 | ✅ |
| **☸️ K8s 多集群** | 🌐 集群管理 | 多集群统一管理，支持 Token/Kubeconfig 认证 | ✅ |
| | 🔐 集群权限 | 基于用户/角色的集群 RBAC，支持命名空间隔离，K8s 级权限规则 | ✅ |
| | 📦 工作负载 | Deployment、DaemonSet、StatefulSet、Pod、CronJob、HPA 管理 | ✅ |
| | ⚙️ 配置管理 | ConfigMap、Secret 的统一管理和编辑 | ✅ |
| | 🌐 服务管理 | Service、Ingress 的创建和管理 | ✅ |
| | 💾 存储管理 | PV、PVC、StorageClass 的配置和管理 | ✅ |
| | 📊 集群监控 | 集群状态概览、资源使用监控、事件查看、Pod 监控（CPU/内存） | ✅ |
| | 📋 操作审计 | K8s 操作的完整审计日志记录 | ✅ |
| | 🔍 全局搜索 | 跨集群全局资源搜索 | ✅ |
| | 📜 YAML 管理 | 资源 YAML 创建/编辑/删除/dry-run | ✅ |
| | 🚢 应用部署 | 应用部署管理，支持版本回滚 | ✅ |
| | 💻 Pod 终端 | WebSocket 在线 Pod 终端与日志流 | ✅ |
| **📋 工单与流程** | 📝 工单创建 | 支持日常工单、发布工单等多种类型 | ✅ |
| | 📑 表单模板 | 可视化表单设计器，支持文本、下拉、日期、表格等字段类型 | ✅ |
| | 📂 表单分类 | 表单模板分类管理 | ✅ |
| | 🔄 审批流程 | 多级审批，支持飞书/钉钉/企微/内部审批 | ✅ |
| | 🔄 工作流引擎 | 自定义工作流，支持多节点、多审批人配置 | ✅ |
| | ✅ 自动授权 | 审批通过后自动应用权限规则，授权主机访问 | ✅ |
| | 📊 工单统计 | 工单状态跟踪、审批历史、统计分析 | ✅ |
| **🏢 组织应用** | 👥 部门管理 | 多级树形部门结构管理 | ✅ |
| | 📱 应用管理 | 应用注册管理，关联部门和人员 | ✅ |
| | 👤 人员管理 | 用户信息管理，支持部门关联和角色分配 | ✅ |
| | 🔧 服务管理 | 服务目录管理，支持服务分类和详情配置 | ✅ |
| | 🔗 应用部署绑定 | 应用与部署关联，用于发布管理 | ✅ |
| | 📦 仓库管理 | 容器镜像仓库集成：Harbor、AWS ECR、Sonatype Nexus | ✅ |
| **🔐 多态权限** | 👥 用户组（角色） | 基于角色的权限管理，支持角色成员增删改查 | ✅ |
| | 🖥️ 主机组 | 主机分组管理，支持主机组权限批量授权 | ✅ |
| | 👤 系统用户 | 系统用户与权限规则关联，支持多对多关系 | ✅ |
| | ⏰ 时间限制 | 权限规则支持时间范围限制（有效起止时间） | ✅ |
| | 🎯 优先级控制 | 权限规则支持优先级设置，高优先级规则优先匹配 | ✅ |
| | 📍 细粒度权限 | 支持主机组、指定主机、系统用户的多维度权限组合 | ✅ |
| | 🗂️ 菜单与 API 权限 | 基于角色的菜单可见性和 API 接口访问控制（Casbin） | ✅ |
| **📈 监控告警** | 📊 Prometheus 监控 | Prometheus 数据源集成，多实例支持，指标查询 | ✅ |
| | 📋 告警规则 | PromQL 告警规则管理，表格支持固定列、横向滚动 | ✅ |
| | 📋 规则组 | 规则组管理，详情页左侧菜单高亮，支持将现有规则加入本组 | ✅ |
| | 🎯 告警策略 | 聚合策略、抑制策略、静默策略配置 | ✅ |
| | 📢 告警通知 | 多渠道通知：飞书、钉钉、邮件、Webhook，支持模板格式化 | ✅ |
| | 📝 告警模板 | 自定义告警消息模板，支持变量替换 | ✅ |
| | 📊 告警事件 | 全生命周期管理：触发 → 确认 → 恢复；事件详情和历史 | ✅ |
| | 🔔 证书监控 | SSL/TLS 证书过期监控，支持域名、SSL、托管证书类型 | ✅ |
| | 👨‍💼 值班管理 | 排班管理，值班日历，自动/手动告警分配 | ✅ |
| | 📈 告警统计 | 告警趋势、级别分布、策略效果分析 | ✅ |
| | 🔗 Prometheus Webhook | 原生 Prometheus Alertmanager webhook 接收 | ✅ |
| **💾 数据库管理** | 🗄️ 多数据库支持 | MySQL、PostgreSQL、MongoDB、Redis 统一管理 | ✅ |
| | 🔍 查询执行 | SQL 查询、MongoDB 查询、Redis 命令执行，结果格式化 | ✅ |
| | 📝 查询审计日志 | 完整查询审计记录：用户、时间、IP、执行 SQL | ✅ |
| | 🔐 细粒度权限 | 基于 Casbin 的权限控制（实例→数据库→表→权限类型） | ✅ |
| | 🧪 连接测试 | 保存前连接验证 | ✅ |
| **☁️ 云费用 FinOps** | 💳 云账号 | 多云账号凭据管理：AWS、阿里云、腾讯云 | ✅ |
| | 📊 费用看板 | 多云费用总览、趋势、对比 | ✅ |
| | 📈 费用拆分 | 按标签、账号、区域、服务、资源拆分 | ✅ |
| | 📉 优化建议 | 基于用量分析的费用优化建议 | ✅ |
| | 📋 资源拆分 | 资源数量与费用分布分析 | ✅ |
| | 🔄 账单同步 | 定时自动同步云账单，支持配置频率 | ✅ |
| **🔧 CMDB MCP** | 🖥️ CMDB 工具 | 通过 Model Context Protocol 暴露 CMDB（主机/资产）查询工具 | ✅ |
| | 🛠️ K8s 工具 | 通过 MCP 暴露 K8s 资源操作工具 | ✅ |
| | 🔌 MCP 服务 | 标准 MCP 服务器，供 AI 工具调用，支持 API 密钥认证 | ✅ |
| **📋 审计** | 📝 操作日志 | 完整的 API 操作审计追溯：用户、动作、资源、时间 | ✅ |
| | 🗃️ Pod 命令审计 | 堡垒机 Pod 命令记录与审计 | ✅ |
| | 🗑️ 日志管理 | 批量删除和保留策略 | ✅ |
| **🔧 基础设施** | 🌐 高可用 | 多实例部署，Redis 分布式锁，配置同步 | ✅ |
| | 📊 资产同步 | Prometheus 资产自动同步，主机信息定时更新 | ✅ |
| | 🔍 主机监控 | 主机在线状态实时监控，健康检查，连通性探测 | ✅ |
| | 🚀 代理注册 | 动态代理注册、心跳、健康监控 | ✅ |
| | 🔔 通知中心 | 集中通知管理：飞书、钉钉、企业微信 | ✅ |
| | 🚦 熔断机制 | 代理故障自动下线，冗余路由 | ✅ |
 ## 快速部署

### 环境要求

- Docker 20.10+
- Docker Compose 2.0+

### MySQL 部署（推荐）

```bash
# 启动所有服务
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down
```

**访问系统**: http://localhost:8080  
**默认账号**: `admin` / `admin123`

### PostgreSQL 部署

修改环境变量，在 `.env` 文件中设置：

```bash
docker-compose -f docker-compose-pg.yaml up -d

DB_DRIVER=postgres
DB_HOST=postgres
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=keyops
```

## 端口说明

- `8080`: HTTP（Web + API）
- `2222`: SSH Gateway
- `3306`: MySQL（可选）
- `5432`: PostgreSQL（可选）
- `6379`: Redis（可选）
- `27017`: MongoDB（可选）
- `4822`: Guacamole daemon（RDP）

## 环境变量配置

创建 `.env` 文件（可选）：

```bash
# 数据库配置
MYSQL_ROOT_PASSWORD=123456
MYSQL_DATABASE=keyops
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=keyops

# Redis 配置
REDIS_ENABLED=true
REDIS_PASSWORD=

# MongoDB 配置
MONGO_INITDB_ROOT_USERNAME=admin
MONGO_INITDB_ROOT_PASSWORD=123456
MONGO_BASTION_URI=mongodb://admin:123456@mongodb:27017/keyops_bastion?authSource=admin
MONGO_BILL_URI=mongodb://admin:123456@mongodb:27017/keyops_bill?authSource=admin

# AI 助手
# LLM/模型配置改为数据库管理（ai_assistant_models），不再通过 .env 配置。
# 前端可选覆盖地址：
# VITE_AI_ASSISTANT_API_URL=http://localhost:8080/api/ai-assistant

# 认证覆盖（紧急场景）
# AUTH_METHOD=local
# ADMIN_WHITELIST=admin@example.com
```

## License

本项目采用 MIT 许可证，详见 [LICENSE](LICENSE)。
