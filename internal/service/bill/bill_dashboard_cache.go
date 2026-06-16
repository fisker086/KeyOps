package bill

import (
	"sync"
	"time"
)

const dashboardCacheTTL = 2 * time.Minute
const summaryCacheTTL = 5 * time.Minute
const recommendationsCacheTTL = 5 * time.Minute

type dashboardCacheEntry struct {
	data      *DashboardData
	expiresAt time.Time
}

type summaryCacheEntry struct {
	data      *DashboardSummaryData
	expiresAt time.Time
}

type chartsCacheEntry struct {
	data      *DashboardChartsData
	expiresAt time.Time
}

type trendCacheEntry struct {
	data      *DashboardTrendData
	expiresAt time.Time
}

type topResourcesCacheEntry struct {
	data      []TopResource
	expiresAt time.Time
}

type recommendationsCacheEntry struct {
	data      []Recommendation
	expiresAt time.Time
}

type recommendationsLoadState struct {
	done chan struct{}
	data []Recommendation
	err  error
}

var (
	dashboardCacheMu           sync.Mutex
	dashboardCacheByKey        = make(map[string]dashboardCacheEntry)
	summaryCacheMu             sync.Mutex
	summaryCacheByKey          = make(map[string]summaryCacheEntry)
	chartsCacheMu              sync.Mutex
	chartsCacheByKey           = make(map[string]chartsCacheEntry)
	trendCacheMu               sync.Mutex
	trendCacheByKey            = make(map[string]trendCacheEntry)
	topResourcesCacheMu        sync.Mutex
	topResourcesCacheKey       = make(map[string]topResourcesCacheEntry)
	recommendationsCacheMu     sync.Mutex
	recommendationsCacheByKey  = make(map[string]recommendationsCacheEntry)
	recommendationsInflightMu  sync.Mutex
	recommendationsInflightReq *recommendationsLoadState
)

func dashboardCacheBucket() string {
	return time.Now().Truncate(dashboardCacheTTL).Format(time.RFC3339)
}

func getCachedDashboard(currency string) (*DashboardData, bool) {
	key := currency + ":" + dashboardCacheBucket()
	dashboardCacheMu.Lock()
	defer dashboardCacheMu.Unlock()
	entry, ok := dashboardCacheByKey[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

func setCachedDashboard(currency string, data *DashboardData) {
	key := currency + ":" + dashboardCacheBucket()
	dashboardCacheMu.Lock()
	defer dashboardCacheMu.Unlock()
	dashboardCacheByKey[key] = dashboardCacheEntry{
		data:      data,
		expiresAt: time.Now().Add(dashboardCacheTTL),
	}
}

func getCachedTopResources(month, currency string) ([]TopResource, bool) {
	key := month + ":" + currency + ":" + dashboardCacheBucket()
	topResourcesCacheMu.Lock()
	defer topResourcesCacheMu.Unlock()
	entry, ok := topResourcesCacheKey[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

func setCachedTopResources(month, currency string, data []TopResource) {
	key := month + ":" + currency + ":" + dashboardCacheBucket()
	topResourcesCacheMu.Lock()
	defer topResourcesCacheMu.Unlock()
	topResourcesCacheKey[key] = topResourcesCacheEntry{
		data:      data,
		expiresAt: time.Now().Add(dashboardCacheTTL),
	}
}

func summaryCacheBucket() string {
	return time.Now().Truncate(summaryCacheTTL).Format(time.RFC3339)
}

func getCachedSummary(currency string) (*DashboardSummaryData, bool) {
	key := currency + ":" + summaryCacheBucket()
	summaryCacheMu.Lock()
	defer summaryCacheMu.Unlock()
	entry, ok := summaryCacheByKey[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

func setCachedSummary(currency string, data *DashboardSummaryData) {
	key := currency + ":" + summaryCacheBucket()
	summaryCacheMu.Lock()
	defer summaryCacheMu.Unlock()
	summaryCacheByKey[key] = summaryCacheEntry{data: data, expiresAt: time.Now().Add(summaryCacheTTL)}
}

func getCachedCharts(currency string) (*DashboardChartsData, bool) {
	key := currency + ":" + dashboardCacheBucket()
	chartsCacheMu.Lock()
	defer chartsCacheMu.Unlock()
	entry, ok := chartsCacheByKey[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

func setCachedCharts(currency string, data *DashboardChartsData) {
	key := currency + ":" + dashboardCacheBucket()
	chartsCacheMu.Lock()
	defer chartsCacheMu.Unlock()
	chartsCacheByKey[key] = chartsCacheEntry{data: data, expiresAt: time.Now().Add(dashboardCacheTTL)}
}

func getCachedTrend(currency string) (*DashboardTrendData, bool) {
	key := currency + ":" + dashboardCacheBucket()
	trendCacheMu.Lock()
	defer trendCacheMu.Unlock()
	entry, ok := trendCacheByKey[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

func setCachedTrend(currency string, data *DashboardTrendData) {
	key := currency + ":" + dashboardCacheBucket()
	trendCacheMu.Lock()
	defer trendCacheMu.Unlock()
	trendCacheByKey[key] = trendCacheEntry{data: data, expiresAt: time.Now().Add(dashboardCacheTTL)}
}

func getCachedRecommendations() ([]Recommendation, bool) {
	key := recommendationsCacheBucket()
	recommendationsCacheMu.Lock()
	defer recommendationsCacheMu.Unlock()
	entry, ok := recommendationsCacheByKey[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

func setCachedRecommendations(data []Recommendation) {
	key := recommendationsCacheBucket()
	recommendationsCacheMu.Lock()
	defer recommendationsCacheMu.Unlock()
	recommendationsCacheByKey[key] = recommendationsCacheEntry{data: data, expiresAt: time.Now().Add(recommendationsCacheTTL)}
}

func recommendationsCacheBucket() string {
	return time.Now().Truncate(recommendationsCacheTTL).Format(time.RFC3339)
}

// acquireRecommendationsInflight 冷缓存时合并并发请求，只跑一次慢查询。
func acquireRecommendationsInflight() (leader bool, wait <-chan struct{}) {
	recommendationsInflightMu.Lock()
	defer recommendationsInflightMu.Unlock()
	if recommendationsInflightReq != nil {
		return false, recommendationsInflightReq.done
	}
	inf := &recommendationsLoadState{done: make(chan struct{})}
	recommendationsInflightReq = inf
	return true, inf.done
}

// InvalidateDashboardCache 清除所有 Dashboard 内存缓存，在数据同步完成后调用。
func InvalidateDashboardCache() {
	dashboardCacheMu.Lock()
	dashboardCacheByKey = make(map[string]dashboardCacheEntry)
	dashboardCacheMu.Unlock()

	summaryCacheMu.Lock()
	summaryCacheByKey = make(map[string]summaryCacheEntry)
	summaryCacheMu.Unlock()

	chartsCacheMu.Lock()
	chartsCacheByKey = make(map[string]chartsCacheEntry)
	chartsCacheMu.Unlock()

	trendCacheMu.Lock()
	trendCacheByKey = make(map[string]trendCacheEntry)
	trendCacheMu.Unlock()

	topResourcesCacheMu.Lock()
	topResourcesCacheKey = make(map[string]topResourcesCacheEntry)
	topResourcesCacheMu.Unlock()

	recommendationsCacheMu.Lock()
	recommendationsCacheByKey = make(map[string]recommendationsCacheEntry)
	recommendationsCacheMu.Unlock()
}

func finishRecommendationsInflight(data []Recommendation, err error) {
	recommendationsInflightMu.Lock()
	defer recommendationsInflightMu.Unlock()
	if recommendationsInflightReq != nil {
		recommendationsInflightReq.data = data
		recommendationsInflightReq.err = err
		close(recommendationsInflightReq.done)
		recommendationsInflightReq = nil
	}
}
