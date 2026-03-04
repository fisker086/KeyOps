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

## 参数兜底与默认动作（强约束）

- 未提供 `uid` 时，必须先 `list_all_dashboards`，再让用户确认目标仪表盘
- 同名或近似仪表盘多条命中时，先给候选列表（title + uid），不自行拍板
- 提取到多个 PromQL 时，优先选择与用户问题最相关的面板查询并说明选择理由

## 失败重试与降级策略

1. `get_dashboard_metadata` 失败时，先确认 uid 是否仍有效（可能已删除/迁移）
2. 若 Grafana API 不可用，降级到 Prometheus 技能：给出可替代的 PromQL 草案
3. 禁止虚构仪表盘或 PromQL；未获取到时明确说明“未检索到可用面板/查询”

## 标准输出结构（建议统一）

1. **结论**：找到哪些相关仪表盘/是否提取到可用查询
2. **证据**：仪表盘 title、uid、匹配标签、提取到的核心 PromQL
3. **下一步**：推荐执行的 `execute_promql_query` 参数（query/duration/step）

## 与 Prometheus 技能配合

Grafana 技能通常与 Prometheus 技能配合使用：Grafana 负责发现和提取查询，Prometheus 负责执行。若环境同时配置了两者，可先查 Grafana 再查 Prometheus。

## 激活条件

目标环境已配置 `graf_url` 和 `graf_token`（Grafana API Token，需 Viewer 及以上权限）。

## 角色化评审要求（发布前）

- 适用角色：`inspector`、`sre`
- 评审模式：
  - `inspector`：走极速评审（2-3 分钟）
  - `sre`：在极速评审基础上，额外检查“证据链完整”
- 必查三板斧：
  1. 参数兜底与默认值
  2. 失败重试与降级策略
  3. 输出结构（结论/证据/下一步）
- `sre` 额外必查：
  - 仪表盘筛选/PromQL 提取结论是否可被 title、uid、标签、查询语句复核
