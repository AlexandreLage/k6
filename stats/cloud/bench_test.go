/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package cloud

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/mailru/easyjson"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/netext/httpext"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/stats"
)

func BenchmarkAggregateHTTP(b *testing.B) {
	script := &loader.SourceData{
		Data: []byte(""),
		URL:  &url.URL{Path: "/script.js"},
	}

	options := lib.Options{
		Duration: types.NullDurationFrom(1 * time.Second),
	}

	config := NewConfig().Apply(Config{
		NoCompress:              null.BoolFrom(true),
		AggregationCalcInterval: types.NullDurationFrom(time.Millisecond * 200),
		AggregationPeriod:       types.NullDurationFrom(time.Millisecond * 200),
	})
	collector, err := New(config, script, options, []lib.ExecutionStep{}, "1.0")
	require.NoError(b, err)
	now := time.Now()
	collector.referenceID = "something"
	containersCount := 500000

	for _, tagCount := range []int{1, 5, 35, 315, 3645} {
		tagCount := tagCount
		b.Run(fmt.Sprintf("tags:%d", tagCount), func(b *testing.B) {
			b.ResetTimer()
			for s := 0; s < b.N; s++ {
				b.StopTimer()
				container := make([]stats.SampleContainer, containersCount)
				for i := 1; i <= containersCount; i++ {
					status := "200"
					if i%tagCount%7 == 6 {
						status = "404"
					} else if i%tagCount%7 == 5 {
						status = "500"
					}

					tags := generateTags(i, tagCount, map[string]string{"status": status})
					container[i-1] = generateHTTPExtTrail(now, time.Duration(i), tags)
				}
				collector.Collect(container)
				b.StartTimer()
				collector.aggregateHTTPTrails(time.Millisecond * 200)
				collector.bufferSamples = nil
			}
		})
	}
}

func generateTags(i, tagCount int, additionals ...map[string]string) *stats.SampleTags {
	res := map[string]string{
		"test": "mest", "a": "b",
		"custom": fmt.Sprintf("group%d", i%tagCount%9),
		"group":  fmt.Sprintf("group%d", i%tagCount%5),
		"url":    fmt.Sprintf("something%d", i%tagCount%11),
		"name":   fmt.Sprintf("else%d", i%tagCount%11),
	}
	for _, a := range additionals {
		for k, v := range a {
			res[k] = v
		}
	}
	return stats.IntoSampleTags(&res)
}

func BenchmarkMetricMarshal(b *testing.B) {
	for _, count := range []int{10000, 100000, 500000} {
		count := count
		b.Run(fmt.Sprintf("%d", count), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				s := generateSamples(count)
				b.StartTimer()
				r, err := easyjson.Marshal(samples(s))
				require.NoError(b, err)
				b.SetBytes(int64(len(r)))
			}
		})
	}
}

func generateSamples(count int) []*Sample {
	samples := make([]*Sample, count)
	now := time.Now()
	for i := range samples {
		tags := generateTags(i, 200)
		switch i % 3 {
		case 0:
			samples[i] = &Sample{
				Type:   DataTypeSingle,
				Metric: "something",
				Data: &SampleDataSingle{
					Time:  toMicroSecond(now),
					Type:  stats.Counter,
					Tags:  tags,
					Value: float64(i),
				},
			}
		case 1:
			aggrData := &SampleDataAggregatedHTTPReqs{
				Time: toMicroSecond(now),
				Type: "aggregated_trend",
				Tags: tags,
			}
			trail := generateHTTPExtTrail(now, time.Duration(i), tags)
			aggrData.Add(trail)
			aggrData.Add(trail)
			aggrData.Add(trail)
			aggrData.Add(trail)
			aggrData.Add(trail)
			aggrData.CalcAverages()

			samples[i] = &Sample{
				Type:   DataTypeAggregatedHTTPReqs,
				Metric: "something",
				Data:   aggrData,
			}
		default:
			samples[i] = NewSampleFromTrail(generateHTTPExtTrail(now, time.Duration(i), tags))
		}
	}
	return samples
}

func generateHTTPExtTrail(now time.Time, i time.Duration, tags *stats.SampleTags) *httpext.Trail {
	return &httpext.Trail{
		Blocked:        i % 200 * 100 * time.Millisecond,
		Connecting:     i % 200 * 200 * time.Millisecond,
		TLSHandshaking: i % 200 * 300 * time.Millisecond,
		Sending:        i % 200 * 400 * time.Millisecond,
		Waiting:        500 * time.Millisecond,
		Receiving:      600 * time.Millisecond,
		EndTime:        now.Add(i % 100 * 100),
		ConnDuration:   500 * time.Millisecond,
		Duration:       i % 150 * 1500 * time.Millisecond,
		Tags:           tags,
	}
}
