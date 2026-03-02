---
name: prometheus
description: Prometheus 指标查询与分析。当目标环境配置了 prom_url 时启用。用于执行 PromQL 查询、搜索指标、获取标签维度、异常检测与相关性分析。
allowed-tools: find_metrics_by_keyword get_metric_dimension execute_promql_query detect_anomaly check_correlation
---

# Prometheus 技能

当用户任务涉及指标查询、PromQL 分析、异常检测、根因定位时，使用本技能。

## 适用场景

- 核心指标巡检（CPU、内存、QPS、错误率等）
- 异常检测与告警分析
- 多指标相关性分析
- 按关键字发现可用指标
- 时间范围查询与趋势分析

## 工具箱详解

### 1. find_metrics_by_keyword

在 Prometheus 中搜索包含关键字的指标名，用于发现可用指标。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| keyword | string | 是 | 搜索关键字，如 "cpu"、"http_requests" |

**示例**：`{"keyword": "http_requests"}`

### 2. get_metric_dimension

获取指定指标名的标签维度，用于下钻分析。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| metric_name | string | 是 | 指标名 |

**示例**：`{"metric_name": "http_requests_total"}`

### 3. execute_promql_query

执行 PromQL 范围查询，返回时序数据。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| query | string | 是 | PromQL 表达式 |
| duration | string | 否 | 时间范围，默认 "1h" |
| step | string | 否 | 步长，默认 "1m" |
| start_time | string | 否 | 开始时间 |
| end_time | string | 否 | 结束时间 |

**示例**：`{"query": "rate(http_requests_total[5m])", "duration": "1h"}`

### 4. detect_anomaly

对 Prometheus 查询结果做 3-sigma 异常检测。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| result_list | array | 是 | execute_promql_query 返回的时序数据 |

### 5. check_correlation

计算两个指标序列的相关系数，用于因果分析。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| result_a | array | 是 | 第一个指标序列 |
| result_b | array | 是 | 第二个指标序列 |

## 使用建议

1. **先搜索再查询**：不确定指标名时，先用 `find_metrics_by_keyword` 搜索
2. **合理设置 duration**：故障排查用 1h，趋势分析可用 24h
3. **异常检测流程**：`execute_promql_query` → `detect_anomaly` → 结合维度下钻
4. **相关性分析**：发现两个指标同时异常时，用 `check_correlation` 验证关联

## 激活条件

目标环境已配置 `prom_url`（从「监控告警 → 数据源」选择 Prometheus/VictoriaMetrics/Thanos）。
