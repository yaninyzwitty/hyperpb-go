// Copyright 2025 Buf Technologies, Inc.
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

package stats

import (
	"slices"
	"sync/atomic"
)

// Median tracks a median statistic.
//
// Must be constructed with [NewMedian]. [Median.Record] may be called
// concurrently, but not with [Median.Get].
type Median struct {
	// Implemented as a ring buffer of samples.
	samples []float64
	w       atomic.Int64 // Offset at which to write the next sample.
	n       atomic.Int64 // Total number of samples ever.
}

// NewMedian returns a new median statistic which remembers the last n samples.
//
// n should be relatively large, at least 100.
func NewMedian(n int) *Median {
	return &Median{samples: make([]float64, n)}
}

// Record records a sample.
func (m *Median) Record(sample float64) {
	// Lock the buffer by setting w to -1.
again:
	w := m.w.Load()
	next := w + 1
	if int(next) == len(m.samples) {
		next = 0
	}
	if !m.w.CompareAndSwap(w, next) {
		goto again
	}
	m.n.Add(1)

	// Technically this may race if len(samples) is small enough and enough
	// goroutines are hammering this value, but the worst that will happen is
	// we get one torn data point that will eventually get overwritten.
	m.samples[w] = sample
}

// Get returns the median value of this statistic.
func (m *Median) Get() float64 {
	samples := m.samples[:min(int(m.n.Load()), len(m.samples))]
	// For now, we copy and sort, but in principle we could also use median
	// of medians to avoid the copy.
	samples = slices.Clone(samples)
	slices.Sort(samples)

	switch {
	case len(samples) == 0:
		return 0
	case len(samples)%2 == 0:
		a := samples[len(samples)/2-1]
		b := samples[len(samples)/2]
		return (a + b) / 2
	default:
		return samples[len(samples)/2]
	}
}
