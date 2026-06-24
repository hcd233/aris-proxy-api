package metrics

import (
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/prometheus/client_golang/prometheus"
	metricpb "github.com/prometheus/client_model/go"
	"github.com/samber/lo"
)

// GatherMetricFamilies 从 Gatherer 采集指标并转换为 DTO
//
//	@param gatherer prometheus.Gatherer
//	@return []dto.MetricFamilyItem
//	@return error
//	@author centonhuang
//	@update 2026-06-23 10:00:00
func GatherMetricFamilies(gatherer prometheus.Gatherer) ([]dto.MetricFamilyItem, error) {
	families, err := gatherer.Gather()
	if err != nil {
		return nil, err
	}
	return lo.Map(families, func(f *metricpb.MetricFamily, _ int) dto.MetricFamilyItem {
		return dto.MetricFamilyItem{
			Name:    f.GetName(),
			Type:    f.GetType().String(),
			Help:    f.GetHelp(),
			Samples: convertMetricSamples(f),
		}
	}), nil
}

func convertMetricSamples(f *metricpb.MetricFamily) []dto.MetricSampleItem {
	metrics := f.GetMetric()
	if len(metrics) == 0 {
		return nil
	}
	return lo.Map(metrics, func(m *metricpb.Metric, _ int) dto.MetricSampleItem {
		labels := lo.SliceToMap(m.GetLabel(), func(l *metricpb.LabelPair) (string, string) {
			return l.GetName(), l.GetValue()
		})
		value := getMetricValue(m)
		return dto.MetricSampleItem{
			Labels: labels,
			Value:  value,
		}
	})
}

func getMetricValue(m *metricpb.Metric) float64 {
	switch {
	case m.GetCounter() != nil:
		return m.GetCounter().GetValue()
	case m.GetGauge() != nil:
		return m.GetGauge().GetValue()
	case m.GetHistogram() != nil:
		return m.GetHistogram().GetSampleSum()
	case m.GetSummary() != nil:
		return m.GetSummary().GetSampleSum()
	case m.GetUntyped() != nil:
		return m.GetUntyped().GetValue()
	default:
		return 0
	}
}
