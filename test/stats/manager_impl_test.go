package test_stats

import (
	"fmt"
	"testing"

	gostats "github.com/lyft/gostats"
	gostatsMock "github.com/lyft/gostats/mock"
	"github.com/stretchr/testify/assert"

	"github.com/envoyproxy/ratelimit/src/settings"
	"github.com/envoyproxy/ratelimit/src/stats"
)

func TestEscapingInvalidChartersInMetricName(t *testing.T) {
	mockSink := gostatsMock.NewSink()
	statsStore := gostats.NewStore(mockSink, false)
	statsManager := stats.NewStatManager(statsStore, settings.Settings{})

	tests := []struct {
		name string
		key  string
		want string
	}{
		{
			name: "use not modified key if it does not contain special characters",
			key:  "path_/foo/bar",
			want: "path_/foo/bar",
		},
		{
			name: "escape colon",
			key:  "path_/foo:*:bar",
			want: "path_/foo_*_bar",
		},
		{
			name: "escape pipe",
			key:  "path_/foo|bar|baz",
			want: "path_/foo_bar_baz",
		},
		{
			name: "escape all special characters",
			key:  "path_/foo:bar|baz",
			want: "path_/foo_bar_baz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := statsManager.NewStats(tt.key)
			assert.Equal(t, tt.key, stats.Key)

			stats.TotalHits.Inc()
			statsManager.GetStatsStore().Flush()
			mockSink.AssertCounterExists(t, fmt.Sprintf("ratelimit.service.rate_limit.%s.total_hits", tt.want))
		})
	}
}

func TestPerKeyStats(t *testing.T) {
	tests := []struct {
		name             string
		enablePerKeyStats bool
		keys             []string
		expectedMetrics  []string
	}{
		{
			name:             "per-key stats enabled",
			enablePerKeyStats: true,
			keys:             []string{"key1", "key2", "key3"},
			expectedMetrics:  []string{"key1.total_hits", "key2.total_hits", "key3.total_hits"},
		},
		{
			name:             "per-key stats disabled",
			enablePerKeyStats: false,
			keys:             []string{"key1", "key2", "key3"},
			expectedMetrics:  []string{"all.total_hits"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSink := gostatsMock.NewSink()
			statsStore := gostats.NewStore(mockSink, false)
			statsManager := stats.NewStatManager(statsStore, settings.Settings{
				EnablePerKeyStats: tt.enablePerKeyStats,
			})

			// Create stats for each key and increment counters
			for _, key := range tt.keys {
				stats := statsManager.NewStats(key)
				assert.Equal(t, key, stats.GetKey()) // Original key should be preserved
				stats.TotalHits.Inc()
				stats.OverLimit.Inc()
				stats.NearLimit.Inc()
				stats.WithinLimit.Inc()
				stats.ShadowMode.Inc()
			}

			statsManager.GetStatsStore().Flush()

			// Verify expected metrics exist
			for _, metric := range tt.expectedMetrics {
				mockSink.AssertCounterExists(t, fmt.Sprintf("ratelimit.service.rate_limit.%s", metric))
			}

			// For disabled per-key stats, verify all stats are aggregated
			if !tt.enablePerKeyStats {
				// All metrics should be aggregated under "all"
				mockSink.AssertCounterValue(t, "ratelimit.service.rate_limit.all.total_hits", float64(len(tt.keys)))
				mockSink.AssertCounterValue(t, "ratelimit.service.rate_limit.all.over_limit", float64(len(tt.keys)))
				mockSink.AssertCounterValue(t, "ratelimit.service.rate_limit.all.near_limit", float64(len(tt.keys)))
				mockSink.AssertCounterValue(t, "ratelimit.service.rate_limit.all.within_limit", float64(len(tt.keys)))
				mockSink.AssertCounterValue(t, "ratelimit.service.rate_limit.all.shadow_mode", float64(len(tt.keys)))

				// Verify individual key metrics don't exist
				for _, key := range tt.keys {
					mockSink.AssertCounterNotExists(t, fmt.Sprintf("ratelimit.service.rate_limit.%s.total_hits", key))
				}
			}
		})
	}
}
