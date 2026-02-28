# KeyOps - 基础设施管理平台

**中文**（默认）| [English](README.en.md)

---

**相关截图**
<img width="2924" height="1374" alt="image" src="https://github.com/user-attachments/assets/8a50b150-3c33-49df-b201-5c398a03f3ab" />
<img width="2504" height="1582" alt="image" src="https://github.com/user-attachments/assets/c20edb04-d634-43a5-94f4-1a982c55e8e7" />

**基于 Go 的企业级 DevOps 一体化平台**

## 核心功能

### 功能概览表

| 功能分类 | 功能名称 | 功能描述 | 状态 |
|---------|---------|---------|------|
| **🛡️ 堡垒机** | 🔐 SSH Gateway | 标准 SSH 协议直连，支持传统 SSH 客户端工具 | ✅ |
| | 🌐 Web Terminal | WebSocket 实时终端，无需安装客户端，支持多会话管理 | ✅ |
| | 🖥️ RDP 图形化 | Windows 远程桌面连接，支持图形界面操作 | ✅ |
| | 🎥 会话录制 | 完整的会话录制和回放功能，支持 Asciinema 格式 | ✅ |
| | 📝 命令历史 | 完整的命令执行历史记录和查询 | ✅ |
| | 📁 文件传输 | 文件上传/下载管理，支持 SFTP 协议 | ✅ |
| | 🚨 命令拦截 | 实时检测危险命令，支持命令黑名单，飞书/钉钉告警 | ✅ |
| | 👤 系统用户管理 | 系统用户（跳板用户）的统一管理和密钥分发 | ✅ |
| | 🔑 双因子认证 | 支持密码 / SSH 密钥等多种认证方式 | ✅ |
| **🤖 AI 助手** | 🤖 智能对话 | 自然语言运维助手，集成 Prometheus/Grafana/K8s 等工具集，多轮对话与上下文 | ✅ |
| | 📋 会话管理 | 会话列表、历史记录、多会话切换 | ✅ |
| | ⏰ 定时任务 | 定时触发专家对话与任务执行 | ✅ |
| **☸️ K8s 多集群** | 🌐 集群管理 | 多集群统一管理，支持 Token/Kubeconfig 认证 | ✅ |
| | 🔐 集群权限 | 基于用户/角色的集群访问权限控制，支持命名空间隔离 | ✅ |
| | 📦 工作负载 | Deployment、DaemonSet、StatefulSet、Pod、CronJob 管理 | ✅ |
| | ⚙️ 配置管理 | ConfigMap、Secret 的统一管理和编辑 | ✅ |
| | 🌐 服务管理 | Service、Ingress 的创建和管理 | ✅ |
| | 💾 存储管理 | PV、PVC、StorageClass 的配置和管理 | ✅ |
| | 📊 集群监控 | 集群状态概览、资源使用监控、事件查看 | ✅ |
| | 📋 操作审计 | K8s 操作的完整审计日志 | ✅ |
| **📋 工单管理** | 📝 工单创建 | 支持日常工单、发布工单等多种类型 | ✅ |
| | 📑 表单模板 | 可视化表单设计器，支持自定义表单模板 | ✅ |
| | 🔄 审批流程 | 支持飞书/钉钉/企微/内部审批，多级审批流程（企微回调待完善） | ✅ |
| | ✅ 自动授权 | 审批通过后自动授权，支持权限规则自动应用 | ✅ |
| | 📊 工单统计 | 工单状态跟踪、审批历史、统计分析 | ✅ |
| **🏢 组织应用** | 👥 部门管理 | 多级部门结构管理，支持部门树形组织 | ✅ |
| | 📱 应用管理 | 应用信息管理，关联部门和人员 | ✅ |
| | 👤 人员管理 | 用户信息管理，支持部门关联和角色分配 | ✅ |
| | 🔧 服务管理 | 服务目录管理，支持服务分类和详情配置 | ✅ |
| **🔐 多态权限** | 👥 用户组（角色） | 基于角色的权限管理，支持角色成员管理 | ✅ |
| | 🖥️ 主机组 | 主机分组管理，支持主机组权限批量授权 | ✅ |
| | 👤 系统用户 | 系统用户与权限规则的关联，支持多对多关系 | ✅ |
| | ⏰ 时间限制 | 权限规则支持时间范围限制（有效起止时间） | ✅ |
| | 🎯 优先级控制 | 权限规则支持优先级设置，高优先级规则优先匹配 | ✅ |
| | 📍 细粒度权限 | 支持主机组、指定主机、系统用户的多维度权限组合 | ✅ |
| **📈 监控告警** | 📊 Prometheus 监控 | Prometheus 数据源集成，支持监控指标查询 | ✅ |
| | 📋 告警规则 | 告警规则管理，支持 PromQL 表达式；表格支持固定列、横向滚动、过长省略号悬停 | ✅ |
| | 📋 规则组 | 规则组管理，详情页左侧菜单高亮；支持将现有规则加入本组（列表与分页一致） | ✅ |
| | 🎯 告警策略 | 告警策略配置，支持告警聚合、抑制、静默 | ✅ |
| | 📢 告警通知 | 多渠道告警通知（飞书/钉钉/邮件/Webhook） | ✅ |
| | 📝 告警模板 | 自定义告警消息模板，支持变量替换 | ✅ |
| | 📊 告警事件 | 告警事件管理，支持告警确认、处理、恢复 | ✅ |
| | 🔔 证书监控 | SSL 证书过期监控和告警 | ✅ |
| | 👨‍💼 值班管理 | OnCall 排班管理，支持值班日历和通知 | ✅ |
| **💾 数据库管理** | 🗄️ 多数据库支持 | MySQL、PostgreSQL、MongoDB、Redis 统一管理 | ✅ |
| | 🔍 查询功能 | SQL 查询、MongoDB 查询、Redis 命令执行 | ✅ |
| | 📝 查询日志 | 完整的查询审计日志，记录用户、时间、IP | ✅ |
| | 🔐 细粒度权限 | 基于 Casbin 的权限控制（实例→数据库→表→权限类型） | ✅ |
| **🔧 基础设施** | 🌐 高可用 | 支持多实例部署，Redis 分布式锁，配置同步 | ✅ |
| | 📊 资产同步 | Prometheus 资产自动同步，主机信息自动更新 | ✅ |
| | 🔍 主机监控 | 主机在线状态实时监控，健康检查 | ✅ |

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

**修改环境变量**，在 `.env` 文件中设置：

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

# AI 助手（可选，启用后需配置大模型）
AI_ASSISTANT_ENABLED=true
AI_ASSISTANT_LLM_API_KEY=sk-xxx
AI_ASSISTANT_LLM_BASE_URL=https://...
AI_ASSISTANT_LLM_MODEL=qwen-max
# AI_ASSISTANT_LLM_MAX_STEPS=30
# AI_ASSISTANT_LLM_PROXY_URL=http://127.0.0.1:7890
```

## License

本项目采用 MIT 许可证，详见 [LICENSE](LICENSE)。
