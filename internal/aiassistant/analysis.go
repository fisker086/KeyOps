package aiassistant

import (
	"fmt"
	"math"
)

// DetectAnomaly 3-sigma 异常检测，输入为 execute_promql_query 或 summarize 的 result_list
func DetectAnomaly(resultList interface{}) interface{} {
	if resultList == nil {
		return "No data for anomaly detection."
	}
	if summary, ok := resultList.(map[string]interface{}); ok {
		if s, ok := summary["summary"].([]interface{}); ok {
			resultList = s
		}
	}
	list, ok := resultList.([]interface{})
	if !ok {
		return "Invalid data format for anomaly detection. Expected a list of series."
	}
	var reports []map[string]interface{}
	for _, r := range list {
		series, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		values := series["values"]
		if values == nil {
			values = series["sampled_values"]
		}
		vals, ok := values.([]interface{})
		if !ok || len(vals) < 3 {
			continue
		}
		var nums []float64
		for _, v := range vals {
			pair, ok := v.([]interface{})
			if !ok || len(pair) < 2 {
				continue
			}
			f, _ := toFloat64(pair[1])
			nums = append(nums, f)
		}
		if len(nums) < 3 {
			continue
		}
		mean, std := meanStd(nums)
		anomalyCount := 0
		for _, n := range nums {
			if math.Abs(n-mean) > 3*std {
				anomalyCount++
			}
		}
		if anomalyCount > 0 {
			minVal, maxVal := nums[0], nums[0]
			for _, n := range nums {
				if n < minVal {
					minVal = n
				}
				if n > maxVal {
					maxVal = n
				}
			}
			reports = append(reports, map[string]interface{}{
				"metric":        series["metric"],
				"anomaly_count": anomalyCount,
				"max_value":     maxVal,
				"min_value":     minVal,
				"mean":          mean,
				"std":           std,
			})
		}
	}
	if len(reports) == 0 {
		return "No significant anomalies detected in the provided series (3-sigma)."
	}
	return reports
}

// CheckCorrelation 计算两条序列的相关系数
func CheckCorrelation(resultA, resultB interface{}) interface{} {
	extractSeries := func(r interface{}) []float64 {
		var list []interface{}
		if m, ok := r.(map[string]interface{}); ok && m["summary"] != nil {
			if s, ok := m["summary"].([]interface{}); ok && len(s) > 0 {
				list = s
			}
		} else if l, ok := r.([]interface{}); ok {
			list = l
		}
		if len(list) == 0 {
			return nil
		}
		first, _ := list[0].(map[string]interface{})
		values := first["values"]
		if values == nil {
			values = first["sampled_values"]
		}
		vals, _ := values.([]interface{})
		var vs []float64
		for _, v := range vals {
			pair, _ := v.([]interface{})
			if len(pair) >= 2 {
				val, _ := toFloat64(pair[1])
				vs = append(vs, val)
			}
		}
		return vs
	}
	vsA := extractSeries(resultA)
	vsB := extractSeries(resultB)
	if len(vsA) == 0 || len(vsB) == 0 {
		return "No data points found in one or both metrics."
	}
	n := len(vsA)
	if len(vsB) < n {
		n = len(vsB)
	}
	if n < 2 {
		return "Not enough overlapping data points for correlation analysis."
	}
	var sumA, sumB, sumAB, sumA2, sumB2 float64
	for i := 0; i < n; i++ {
		a, b := vsA[i], vsB[i]
		sumA += a
		sumB += b
		sumAB += a * b
		sumA2 += a * a
		sumB2 += b * b
	}
	meanA := sumA / float64(n)
	meanB := sumB / float64(n)
	cov := sumAB/float64(n) - meanA*meanB
	stdA := math.Sqrt(sumA2/float64(n) - meanA*meanA)
	stdB := math.Sqrt(sumB2/float64(n) - meanB*meanB)
	if stdA == 0 || stdB == 0 {
		return "Correlation coefficient is NaN (possibly due to constant values)."
	}
	corr := cov / (stdA * stdB)
	if math.IsNaN(corr) {
		return "Correlation coefficient is NaN (possibly due to constant values)."
	}
	return fmt.Sprintf("Correlation coefficient between the two metrics is: %.4f", corr)
}

func meanStd(x []float64) (mean, std float64) {
	if len(x) == 0 {
		return 0, 0
	}
	for _, v := range x {
		mean += v
	}
	mean /= float64(len(x))
	for _, v := range x {
		std += (v - mean) * (v - mean)
	}
	std = math.Sqrt(std / float64(len(x)))
	return mean, std
}

func toFloat64(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case string:
		var f float64
		_, err := fmt.Sscanf(x, "%f", &f)
		return f, err == nil
	}
	return 0, false
}
