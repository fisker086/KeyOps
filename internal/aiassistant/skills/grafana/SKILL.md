---
name: grafana
description: Grafana 仪表盘查询。当目标环境配置了 graf_url 时启用。用于列出仪表盘、获取仪表盘中的 PromQL 查询，便于复用已有面板的查询逻辑。
allowed-tools: list_all_dashboards get_dashboard_metadata
---

# Grafana 技能

当用户任务涉及查看 Grafana 仪表盘、从仪表盘提取 PromQL、复用已有监控面板逻辑时，使用本技能。

## 适用场景

- 了解环境中有哪些仪表盘
- 从仪表盘提取 PromQL 表达式，避免手写
- 巡检时参考已有监控面板的查询
- 故障排查时快速定位相关仪表盘

## 工具箱详解

### 1. list_all_dashboards

获取 Grafana 中所有仪表盘列表，返回 title、uid、tags。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| - | - | - | 无需参数 |

**示例**：`{}`

### 2. get_dashboard_metadata

获取指定 UID 仪表盘中的 PromQL 查询，用于提取可复用的查询表达式。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| uid | string | 是 | 仪表盘 UID，从 list_all_dashboards 获取 |

**示例**：`{"uid": "abc123"}`

## 典型流程

1. 调用 `list_all_dashboards` 获取仪表盘列表
2. 根据 title/tags 筛选相关仪表盘
3. 调用 `get_dashboard_metadata` 提取该仪表盘的 PromQL
4. 将提取的 PromQL 传给 Prometheus 技能的 `execute_promql_query` 执行

## 与 Prometheus 技能配合

Grafana 技能通常与 Prometheus 技能配合使用：Grafana 负责发现和提取查询，Prometheus 负责执行。若环境同时配置了两者，可先查 Grafana 再查 Prometheus。

## 激活条件

目标环境已配置 `graf_url` 和 `graf_token`（Grafana API Token，需 Viewer 及以上权限）。
