// Copyright 2020-2025 Buf Technologies, Inc.
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

package swiss

import (
	"math"
	"testing"

	"github.com/bufbuild/fastpb/internal/flag2"
)

func TestHashQuality(t *testing.T) {
	t.Parallel()
	if flag2.Lookup[string]("test.run") == "" {
		// Don't run the hash quality test unless it's specifically enabled.
		t.SkipNow()
	}

	totals := make([]float64, 64)
	seed := fxhash(0x243f6a8885a308d3) // Deterministic random value (digits of Pi).
	trials := 1000000

	for i := range uint64(trials) {
		v := seed.u64(i)
		for j := range 64 {
			if v&1 == 1 {
				totals[j]++
			} else {
				totals[j]--
			}
			v >>= 1
		}
	}

	for i := range totals {
		totals[i] /= float64(trials)
	}

	var avg float64
	for _, total := range totals {
		avg += total
	}
	avg /= float64(len(totals))

	var variance float64
	for _, total := range totals {
		diff := avg - total
		variance += diff * diff
	}
	variance /= float64(len(totals))
	stddev := math.Sqrt(variance)

	t.Logf("avg: %e, stddev: %e", avg, stddev)
	for i, f := range totals {
		t.Logf("totals[%d]: %g", i, f)
	}
}
