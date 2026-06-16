package bill

import (
	"math"
	"strconv"
	"time"

	"github.com/fisker086/keyops/internal/repository"
)

type Insight struct {
	Severity          string  `json:"severity"`
	Category          string  `json:"category"`
	Title             string  `json:"title"`
	Description       string  `json:"description"`
	EstimatedSave     float64 `json:"estimated_save,omitempty"`
	EstimatedCurrency string  `json:"estimated_currency,omitempty"`
}

// GetInsights 规则式优化洞察 —  Engine.insights()
// 返回一系列基于当前数据的 cost optimization 建议
func (s *BillService) GetInsights() ([]Insight, error) {
	now := time.Now()
	thisMonth := now.Format("2006-01")
	lastMonth := now.AddDate(0, -1, 0).Format("2006-01")

	var out []Insight

	// 1. 服务集中度 — top service 占比过高
	svcResult, err := s.repo.QueryRows(repository.Query{
		Dimensions: []repository.Dimension{repository.DimServiceCode},
		DateFrom:   thisMonth,
		DateTo:     thisMonth,
		SortBy:     "cost",
		Descending: true,
		Limit:      5,
	})
	if err == nil && svcResult != nil && len(svcResult.Rows) > 0 {
		total := svcResult.Total
		if total > 0 {
			top := svcResult.Rows[0]
			share := (top.Cost / total) * 100
			if share >= 40 {
				sev := "warning"
				if share >= 70 {
					sev = "critical"
				}
				out = append(out, Insight{
					Severity:    sev,
					Category:    "concentration",
					Title:       "成本集中度过高",
					Description: top.Group + " 占本月成本的 " + formatPct(share) + "，建议评估是否可拆分或优化",
				})
			}

			// 2. Top 5 之外的长尾成本
			if len(svcResult.Rows) >= 5 {
				top5Total := 0.0
				for _, r := range svcResult.Rows {
					top5Total += r.Cost
				}
				othersTotal := total - top5Total
				if othersTotal > 0 && othersTotal/total > 0.2 {
					out = append(out, Insight{
						Severity:    "info",
						Category:    "long_tail",
						Title:       "成本长尾分布",
						Description: "Top 5 服务之外的" + formatPct(othersTotal/total*100) + "成本来自大量小服务，建议审查是否有不再使用的服务",
					})
				}
			}
		}
	}

	// 3. 成本增长 — 环比上月 prorated 增长 > 30%
	thisCost, _ := s.repo.GetTotalCostByMonth(thisMonth)
	lastCost, _ := s.repo.GetTotalCostByMonth(lastMonth)
	if lastCost > 0 {
		_, lastMonthEnd := getMonthRange(now.AddDate(0, -1, 0))
		daysInLastMonth := lastMonthEnd.Day()
		day := now.Day()
		if day > daysInLastMonth {
			day = daysInLastMonth
		}
		proratedLast := lastCost * float64(day) / float64(daysInLastMonth)
		if proratedLast > 0 {
			change := ((thisCost - proratedLast) / proratedLast) * 100
			if change > 30 {
				out = append(out, Insight{
					Severity:    "warning",
					Category:    "cost_growth",
					Title:       "成本环比大幅增长",
					Description: "本月已用成本较上月同期增长 " + formatPct(change) + "，建议检查是否有新开资源或流量突增",
				})
			} else if change < -30 {
				out = append(out, Insight{
					Severity:    "info",
					Category:    "cost_reduction",
					Title:       "成本环比显著下降",
					Description: "本月已用成本较上月同期下降 " + formatPct(-change) + "，确认是否有资源释放或降配",
				})
			}
		}
	}

	// 4. 新账号检测
	thisVendorResult, err := s.repo.QueryRows(repository.Query{
		Dimensions: []repository.Dimension{repository.DimVendor},
		DateFrom:   thisMonth,
		DateTo:     thisMonth,
		Limit:      50,
	})
	if err == nil && thisVendorResult != nil {
		lastVendorResult, _ := s.repo.QueryRows(repository.Query{
			Dimensions: []repository.Dimension{repository.DimVendor},
			DateFrom:   lastMonth,
			DateTo:     lastMonth,
			Limit:      50,
		})
		thisVendors := make(map[string]bool)
		for _, r := range thisVendorResult.Rows {
			thisVendors[r.Group] = true
		}
		if lastVendorResult != nil {
			for _, r := range lastVendorResult.Rows {
				delete(thisVendors, r.Group)
			}
		}
		for vendor := range thisVendors {
			out = append(out, Insight{
				Severity:    "info",
				Category:    "new_vendor",
				Title:       "新增云厂商",
				Description: "检测到新的云厂商 " + vendor + " 产生费用，请确认是否为预期新增",
			})
		}
	}

	return out, nil
}

func formatPct(v float64) string {
	return strconv.FormatFloat(math.Round(v*10)/10, 'f', 1, 64) + "%"
}
