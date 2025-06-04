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

package stats_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bufbuild/fastpb/internal/stats"
)

func TestMean(t *testing.T) {
	t.Parallel()

	m := new(stats.Mean)
	assert.Equal(t, m.Get(), float64(0.0)) //nolint:testifylint

	m.Record(5)
	assert.Equal(t, m.Get(), float64(5.0)) //nolint:testifylint

	m.Record(6)
	assert.Equal(t, m.Get(), float64(5.5)) //nolint:testifylint

	m.Record(-10)
	assert.Equal(t, m.Get(), float64(1)/3) //nolint:testifylint
}
