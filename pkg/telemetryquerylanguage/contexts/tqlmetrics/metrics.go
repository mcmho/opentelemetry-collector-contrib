// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// nolint:gocritic
package tqlmetrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/contexts/tqlmetrics"
import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/contexts/internal/tqlcommon"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"
)

type TransformContext struct {
	dataPoint            interface{}
	metric               pmetric.Metric
	metrics              pmetric.MetricSlice
	instrumentationScope pcommon.InstrumentationScope
	resource             pcommon.Resource
}

func NewTransformContext(dataPoint interface{}, metric pmetric.Metric, metrics pmetric.MetricSlice, instrumentationScope pcommon.InstrumentationScope, resource pcommon.Resource) TransformContext {
	return TransformContext{
		dataPoint:            dataPoint,
		metric:               metric,
		metrics:              metrics,
		instrumentationScope: instrumentationScope,
		resource:             resource,
	}
}

func (ctx TransformContext) GetItem() interface{} {
	return ctx.dataPoint
}

func (ctx TransformContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return ctx.instrumentationScope
}

func (ctx TransformContext) GetResource() pcommon.Resource {
	return ctx.resource
}

func (ctx TransformContext) GetMetric() pmetric.Metric {
	return ctx.metric
}

func (ctx TransformContext) GetMetrics() pmetric.MetricSlice {
	return ctx.metrics
}

var symbolTable = map[tql.EnumSymbol]tql.Enum{
	"AGGREGATION_TEMPORALITY_UNSPECIFIED":    tql.Enum(pmetric.MetricAggregationTemporalityUnspecified),
	"AGGREGATION_TEMPORALITY_DELTA":          tql.Enum(pmetric.MetricAggregationTemporalityDelta),
	"AGGREGATION_TEMPORALITY_CUMULATIVE":     tql.Enum(pmetric.MetricAggregationTemporalityCumulative),
	"FLAG_NONE":                              0,
	"FLAG_NO_RECORDED_VALUE":                 1,
	"METRIC_DATA_TYPE_NONE":                  tql.Enum(pmetric.MetricDataTypeNone),
	"METRIC_DATA_TYPE_GAUGE":                 tql.Enum(pmetric.MetricDataTypeGauge),
	"METRIC_DATA_TYPE_SUM":                   tql.Enum(pmetric.MetricDataTypeSum),
	"METRIC_DATA_TYPE_HISTOGRAM":             tql.Enum(pmetric.MetricDataTypeHistogram),
	"METRIC_DATA_TYPE_EXPONENTIAL_HISTOGRAM": tql.Enum(pmetric.MetricDataTypeExponentialHistogram),
	"METRIC_DATA_TYPE_SUMMARY":               tql.Enum(pmetric.MetricDataTypeSummary),
}

func ParseEnum(val *tql.EnumSymbol) (*tql.Enum, error) {
	if val != nil {
		if enum, ok := symbolTable[*val]; ok {
			return &enum, nil
		}
		return nil, fmt.Errorf("enum symbol, %s, not found", *val)
	}
	return nil, fmt.Errorf("enum symbol not provided")
}

func ParsePath(val *tql.Path) (tql.GetSetter, error) {
	if val != nil && len(val.Fields) > 0 {
		return newPathGetSetter(val.Fields)
	}
	return nil, fmt.Errorf("bad path %v", val)
}

func newPathGetSetter(path []tql.Field) (tql.GetSetter, error) {
	switch path[0].Name {
	case "resource":
		return tqlcommon.ResourcePathGetSetter(path[1:])
	case "instrumentation_scope":
		return tqlcommon.ScopePathGetSetter(path[1:])
	case "metric":
		if len(path) == 1 {
			return accessMetric(), nil
		}
		switch path[1].Name {
		case "name":
			return accessMetricName(), nil
		case "description":
			return accessMetricDescription(), nil
		case "unit":
			return accessMetricUnit(), nil
		case "type":
			return accessMetricType(), nil
		case "aggregation_temporality":
			return accessMetricAggTemporality(), nil
		case "is_monotonic":
			return accessMetricIsMonotonic(), nil
		}
	case "attributes":
		mapKey := path[0].MapKey
		if mapKey == nil {
			return accessAttributes(), nil
		}
		return accessAttributesKey(mapKey), nil
	case "start_time_unix_nano":
		return accessStartTimeUnixNano(), nil
	case "time_unix_nano":
		return accessTimeUnixNano(), nil
	case "value_double":
		return accessDoubleValue(), nil
	case "value_int":
		return accessIntValue(), nil
	case "exemplars":
		return accessExemplars(), nil
	case "flags":
		return accessFlags(), nil
	case "count":
		return accessCount(), nil
	case "sum":
		return accessSum(), nil
	case "bucket_counts":
		return accessBucketCounts(), nil
	case "explicit_bounds":
		return accessExplicitBounds(), nil
	case "scale":
		return accessScale(), nil
	case "zero_count":
		return accessZeroCount(), nil
	case "positive":
		if len(path) == 1 {
			return accessPositive(), nil
		}
		switch path[1].Name {
		case "offset":
			return accessPositiveOffset(), nil
		case "bucket_counts":
			return accessPositiveBucketCounts(), nil
		}
	case "negative":
		if len(path) == 1 {
			return accessNegative(), nil
		}
		switch path[1].Name {
		case "offset":
			return accessNegativeOffset(), nil
		case "bucket_counts":
			return accessNegativeBucketCounts(), nil
		}
	case "quantile_values":
		return accessQuantileValues(), nil
	}
	return nil, fmt.Errorf("invalid path expression %v", path)
}

func accessMetric() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.(TransformContext).GetMetric()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newMetric, ok := val.(pmetric.Metric); ok {
				newMetric.CopyTo(ctx.(TransformContext).GetMetric())
			}
		},
	}
}

func accessMetricName() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.(TransformContext).GetMetric().Name()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.(TransformContext).GetMetric().SetName(str)
			}
		},
	}
}

func accessMetricDescription() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.(TransformContext).GetMetric().Description()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.(TransformContext).GetMetric().SetDescription(str)
			}
		},
	}
}

func accessMetricUnit() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.(TransformContext).GetMetric().Unit()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.(TransformContext).GetMetric().SetUnit(str)
			}
		},
	}
}

func accessMetricType() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return int64(ctx.(TransformContext).GetMetric().DataType())
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			// TODO Implement methods so correctly convert data types.
			// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/10130
		},
	}
}

func accessMetricAggTemporality() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			metric := ctx.(TransformContext).GetMetric()
			switch metric.DataType() {
			case pmetric.MetricDataTypeSum:
				return int64(metric.Sum().AggregationTemporality())
			case pmetric.MetricDataTypeHistogram:
				return int64(metric.Histogram().AggregationTemporality())
			case pmetric.MetricDataTypeExponentialHistogram:
				return int64(metric.ExponentialHistogram().AggregationTemporality())
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newAggTemporality, ok := val.(int64); ok {
				metric := ctx.(TransformContext).GetMetric()
				switch metric.DataType() {
				case pmetric.MetricDataTypeSum:
					metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporality(newAggTemporality))
				case pmetric.MetricDataTypeHistogram:
					metric.Histogram().SetAggregationTemporality(pmetric.MetricAggregationTemporality(newAggTemporality))
				case pmetric.MetricDataTypeExponentialHistogram:
					metric.ExponentialHistogram().SetAggregationTemporality(pmetric.MetricAggregationTemporality(newAggTemporality))
				}
			}
		},
	}
}

func accessMetricIsMonotonic() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			metric := ctx.(TransformContext).GetMetric()
			switch metric.DataType() {
			case pmetric.MetricDataTypeSum:
				return metric.Sum().IsMonotonic()
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newIsMonotonic, ok := val.(bool); ok {
				metric := ctx.(TransformContext).GetMetric()
				switch metric.DataType() {
				case pmetric.MetricDataTypeSum:
					metric.Sum().SetIsMonotonic(newIsMonotonic)
				}
			}
		},
	}
}

func accessAttributes() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				return ctx.GetItem().(pmetric.NumberDataPoint).Attributes()
			case pmetric.HistogramDataPoint:
				return ctx.GetItem().(pmetric.HistogramDataPoint).Attributes()
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Attributes()
			case pmetric.SummaryDataPoint:
				return ctx.GetItem().(pmetric.SummaryDataPoint).Attributes()
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				if attrs, ok := val.(pcommon.Map); ok {
					attrs.CopyTo(ctx.GetItem().(pmetric.NumberDataPoint).Attributes())
				}
			case pmetric.HistogramDataPoint:
				if attrs, ok := val.(pcommon.Map); ok {
					attrs.CopyTo(ctx.GetItem().(pmetric.HistogramDataPoint).Attributes())
				}
			case pmetric.ExponentialHistogramDataPoint:
				if attrs, ok := val.(pcommon.Map); ok {
					attrs.CopyTo(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Attributes())
				}
			case pmetric.SummaryDataPoint:
				if attrs, ok := val.(pcommon.Map); ok {
					attrs.CopyTo(ctx.GetItem().(pmetric.SummaryDataPoint).Attributes())
				}
			}
		},
	}
}

func accessAttributesKey(mapKey *string) tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				return tqlcommon.GetMapValue(ctx.GetItem().(pmetric.NumberDataPoint).Attributes(), *mapKey)
			case pmetric.HistogramDataPoint:
				return tqlcommon.GetMapValue(ctx.GetItem().(pmetric.HistogramDataPoint).Attributes(), *mapKey)
			case pmetric.ExponentialHistogramDataPoint:
				return tqlcommon.GetMapValue(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Attributes(), *mapKey)
			case pmetric.SummaryDataPoint:
				return tqlcommon.GetMapValue(ctx.GetItem().(pmetric.SummaryDataPoint).Attributes(), *mapKey)
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				tqlcommon.SetMapValue(ctx.GetItem().(pmetric.NumberDataPoint).Attributes(), *mapKey, val)
			case pmetric.HistogramDataPoint:
				tqlcommon.SetMapValue(ctx.GetItem().(pmetric.HistogramDataPoint).Attributes(), *mapKey, val)
			case pmetric.ExponentialHistogramDataPoint:
				tqlcommon.SetMapValue(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Attributes(), *mapKey, val)
			case pmetric.SummaryDataPoint:
				tqlcommon.SetMapValue(ctx.GetItem().(pmetric.SummaryDataPoint).Attributes(), *mapKey, val)
			}
		},
	}
}

func accessStartTimeUnixNano() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				return ctx.GetItem().(pmetric.NumberDataPoint).StartTimestamp().AsTime().UnixNano()
			case pmetric.HistogramDataPoint:
				return ctx.GetItem().(pmetric.HistogramDataPoint).StartTimestamp().AsTime().UnixNano()
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).StartTimestamp().AsTime().UnixNano()
			case pmetric.SummaryDataPoint:
				return ctx.GetItem().(pmetric.SummaryDataPoint).StartTimestamp().AsTime().UnixNano()
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newTime, ok := val.(int64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.NumberDataPoint:
					ctx.GetItem().(pmetric.NumberDataPoint).SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				case pmetric.HistogramDataPoint:
					ctx.GetItem().(pmetric.HistogramDataPoint).SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				case pmetric.SummaryDataPoint:
					ctx.GetItem().(pmetric.SummaryDataPoint).SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				}
			}
		},
	}
}

func accessTimeUnixNano() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				return ctx.GetItem().(pmetric.NumberDataPoint).Timestamp().AsTime().UnixNano()
			case pmetric.HistogramDataPoint:
				return ctx.GetItem().(pmetric.HistogramDataPoint).Timestamp().AsTime().UnixNano()
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Timestamp().AsTime().UnixNano()
			case pmetric.SummaryDataPoint:
				return ctx.GetItem().(pmetric.SummaryDataPoint).Timestamp().AsTime().UnixNano()
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newTime, ok := val.(int64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.NumberDataPoint:
					ctx.GetItem().(pmetric.NumberDataPoint).SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				case pmetric.HistogramDataPoint:
					ctx.GetItem().(pmetric.HistogramDataPoint).SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				case pmetric.SummaryDataPoint:
					ctx.GetItem().(pmetric.SummaryDataPoint).SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				}
			}
		},
	}
}

func accessDoubleValue() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				return ctx.GetItem().(pmetric.NumberDataPoint).DoubleVal()
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newDouble, ok := val.(float64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.NumberDataPoint:
					ctx.GetItem().(pmetric.NumberDataPoint).SetDoubleVal(newDouble)
				}
			}
		},
	}
}

func accessIntValue() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				return ctx.GetItem().(pmetric.NumberDataPoint).IntVal()
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newInt, ok := val.(int64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.NumberDataPoint:
					ctx.GetItem().(pmetric.NumberDataPoint).SetIntVal(newInt)
				}
			}
		},
	}
}

func accessExemplars() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				return ctx.GetItem().(pmetric.NumberDataPoint).Exemplars()
			case pmetric.HistogramDataPoint:
				return ctx.GetItem().(pmetric.HistogramDataPoint).Exemplars()
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Exemplars()
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newExemplars, ok := val.(pmetric.ExemplarSlice); ok {
				switch ctx.GetItem().(type) {
				case pmetric.NumberDataPoint:
					newExemplars.CopyTo(ctx.GetItem().(pmetric.NumberDataPoint).Exemplars())
				case pmetric.HistogramDataPoint:
					newExemplars.CopyTo(ctx.GetItem().(pmetric.HistogramDataPoint).Exemplars())
				case pmetric.ExponentialHistogramDataPoint:
					newExemplars.CopyTo(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Exemplars())
				}
			}
		},
	}
}

func accessFlags() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				return int64(ctx.GetItem().(pmetric.NumberDataPoint).Flags().AsRaw())
			case pmetric.HistogramDataPoint:
				return int64(ctx.GetItem().(pmetric.HistogramDataPoint).Flags().AsRaw())
			case pmetric.ExponentialHistogramDataPoint:
				return int64(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Flags().AsRaw())
			case pmetric.SummaryDataPoint:
				return int64(ctx.GetItem().(pmetric.SummaryDataPoint).Flags().AsRaw())
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newFlags, ok := val.(int64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.NumberDataPoint:
					setFlagsValue(ctx.GetItem().(pmetric.NumberDataPoint).Flags(), newFlags)
				case pmetric.HistogramDataPoint:
					setFlagsValue(ctx.GetItem().(pmetric.HistogramDataPoint).Flags(), newFlags)
				case pmetric.ExponentialHistogramDataPoint:
					setFlagsValue(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Flags(), newFlags)
				case pmetric.SummaryDataPoint:
					setFlagsValue(ctx.GetItem().(pmetric.SummaryDataPoint).Flags(), newFlags)
				}
			}
		},
	}
}

func setFlagsValue(flags pmetric.MetricDataPointFlags, value int64) {
	if value&1 != 0 {
		flags.SetNoRecordedValue(true)
	} else {
		flags.SetNoRecordedValue(false)
	}
}

func accessCount() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.HistogramDataPoint:
				return int64(ctx.GetItem().(pmetric.HistogramDataPoint).Count())
			case pmetric.ExponentialHistogramDataPoint:
				return int64(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Count())
			case pmetric.SummaryDataPoint:
				return int64(ctx.GetItem().(pmetric.SummaryDataPoint).Count())
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newCount, ok := val.(int64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.HistogramDataPoint:
					ctx.GetItem().(pmetric.HistogramDataPoint).SetCount(uint64(newCount))
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).SetCount(uint64(newCount))
				case pmetric.SummaryDataPoint:
					ctx.GetItem().(pmetric.SummaryDataPoint).SetCount(uint64(newCount))
				}
			}
		},
	}
}

func accessSum() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.HistogramDataPoint:
				return ctx.GetItem().(pmetric.HistogramDataPoint).Sum()
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Sum()
			case pmetric.SummaryDataPoint:
				return ctx.GetItem().(pmetric.SummaryDataPoint).Sum()
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newSum, ok := val.(float64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.HistogramDataPoint:
					ctx.GetItem().(pmetric.HistogramDataPoint).SetSum(newSum)
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).SetSum(newSum)
				case pmetric.SummaryDataPoint:
					ctx.GetItem().(pmetric.SummaryDataPoint).SetSum(newSum)
				}
			}
		},
	}
}

func accessExplicitBounds() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.HistogramDataPoint:
				return ctx.GetItem().(pmetric.HistogramDataPoint).ExplicitBounds().AsRaw()
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newExplicitBounds, ok := val.([]float64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.HistogramDataPoint:
					ctx.GetItem().(pmetric.HistogramDataPoint).SetExplicitBounds(pcommon.NewImmutableFloat64Slice(newExplicitBounds))
				}
			}
		},
	}
}

func accessBucketCounts() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.HistogramDataPoint:
				return ctx.GetItem().(pmetric.HistogramDataPoint).BucketCounts().AsRaw()
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newBucketCount, ok := val.([]uint64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.HistogramDataPoint:
					ctx.GetItem().(pmetric.HistogramDataPoint).SetBucketCounts(pcommon.NewImmutableUInt64Slice(newBucketCount))
				}
			}
		},
	}
}

func accessScale() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return int64(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Scale())
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newScale, ok := val.(int64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).SetScale(int32(newScale))
				}
			}
		},
	}
}

func accessZeroCount() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return int64(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).ZeroCount())
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newZeroCount, ok := val.(int64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).SetZeroCount(uint64(newZeroCount))
				}
			}
		},
	}
}

func accessPositive() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Positive()
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newPositive, ok := val.(pmetric.Buckets); ok {
				switch ctx.GetItem().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					newPositive.CopyTo(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Positive())
				}
			}
		},
	}
}

func accessPositiveOffset() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return int64(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Positive().Offset())
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newPositiveOffset, ok := val.(int64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Positive().SetOffset(int32(newPositiveOffset))
				}
			}
		},
	}
}

func accessPositiveBucketCounts() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Positive().BucketCounts().AsRaw()
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newPositiveBucketCounts, ok := val.([]uint64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Positive().SetBucketCounts(pcommon.NewImmutableUInt64Slice(newPositiveBucketCounts))
				}
			}
		},
	}
}

func accessNegative() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Negative()
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newNegative, ok := val.(pmetric.Buckets); ok {
				switch ctx.GetItem().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					newNegative.CopyTo(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Negative())
				}
			}
		},
	}
}

func accessNegativeOffset() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return int64(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Negative().Offset())
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newNegativeOffset, ok := val.(int64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Negative().SetOffset(int32(newNegativeOffset))
				}
			}
		},
	}
}

func accessNegativeBucketCounts() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Negative().BucketCounts().AsRaw()
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newNegativeBucketCounts, ok := val.([]uint64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Negative().SetBucketCounts(pcommon.NewImmutableUInt64Slice(newNegativeBucketCounts))
				}
			}
		},
	}
}

func accessQuantileValues() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.SummaryDataPoint:
				return ctx.GetItem().(pmetric.SummaryDataPoint).QuantileValues()
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newQuantileValues, ok := val.(pmetric.ValueAtQuantileSlice); ok {
				switch ctx.GetItem().(type) {
				case pmetric.SummaryDataPoint:
					newQuantileValues.CopyTo(ctx.GetItem().(pmetric.SummaryDataPoint).QuantileValues())
				}
			}
		},
	}
}
