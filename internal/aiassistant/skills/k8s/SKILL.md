---
name: k8s
description: Kubernetes 集群巡检。当目标环境关联了 k8s_cluster_id 时启用。用于获取集群摘要、节点、命名空间、Pod、工作负载及事件，支持故障排查与资源分析。
allowed-tools: k8s_cluster_summary k8s_list_nodes k8s_list_namespaces k8s_list_pods k8s_list_deployments k8s_list_daemonsets k8s_list_statefulsets k8s_list_events
---

# K8s 技能

当用户任务涉及 K8s 集群巡检、节点/Pod 状态、工作负载分析、事件排查、资源使用分析时，使用本技能。

## 适用场景

- 集群健康巡检（节点、Pod、工作负载）
- 故障排查（事件、异常 Pod、调度失败）
- 资源使用分析（结合 Prometheus 的 CPU/内存指标）
- 命名空间/工作负载清单
- 部署状态检查

## 工具箱详解

### 1. k8s_cluster_summary

获取集群摘要：节点数、Pod 数、工作负载数等，用于快速了解集群状态。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| - | - | - | 无需参数 |

**示例**：`{}`

### 2. k8s_list_nodes

列出集群节点及状态（Ready、NotReady 等）。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| - | - | - | 无需参数 |

### 3. k8s_list_namespaces

列出所有命名空间。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| - | - | - | 无需参数 |

### 4. k8s_list_pods

列出指定命名空间下的 Pod，可筛选状态。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| namespace | string | 否 | 命名空间，默认 "default" |

**示例**：`{"namespace": "production"}`

### 5. k8s_list_deployments

列出 Deployment 工作负载。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| namespace | string | 否 | 命名空间 |

### 6. k8s_list_daemonsets

列出 DaemonSet 工作负载。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| namespace | string | 否 | 命名空间 |

### 7. k8s_list_statefulsets

列出 StatefulSet 工作负载。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| namespace | string | 否 | 命名空间 |

### 8. k8s_list_events

列出事件，可按命名空间、对象名、对象类型过滤，用于故障排查。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| namespace | string | 否 | 命名空间 |
| object_name | string | 否 | 对象名 |
| object_kind | string | 否 | 对象类型（Pod、Deployment 等） |

**示例**：`{"namespace": "default", "object_kind": "Pod"}`

## 巡检建议流程

1. **概览**：`k8s_cluster_summary` → `k8s_list_nodes` → `k8s_list_namespaces`
2. **事件优先**：尽早调用 `k8s_list_events` 发现 Critical/Warning
3. **异常定位**：根据事件定位到 Pod/Deployment，再用 `k8s_list_pods` 等查看详情
4. **结合指标**：若环境同时配置 Prometheus，可用 `execute_promql_query` 查 CPU/内存使用

## 与 Prometheus 技能配合

K8s 技能提供集群真实状态（节点、Pod、事件），Prometheus 提供资源指标（CPU、内存、网络）。两者结合可做完整的 K8s 巡检与根因分析。

## 激活条件

目标环境已关联 K8s 集群（`k8s_cluster_id`，从「K8s 管理」选择），且 K8s Runner 已配置。
