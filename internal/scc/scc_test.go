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

package scc_test

import (
	"iter"
	"math"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"buf.build/go/hyperpb/internal/scc"
)

func TestSort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, graph string
		want        [][]int // The expected components.
		deps        [][]int // Outgoing dependencies.
	}{
		{
			name:  "singleton",
			graph: `.`,
			want:  [][]int{{0}},
			deps:  [][]int{{}},
		},
		{
			name:  "loop",
			graph: `#`,
			want:  [][]int{{0}},
			deps:  [][]int{{}},
		},
		{
			name: "tree",
			graph: `.##..
					.....
					...##
					.....
					.....`,
			want: [][]int{{1}, {3}, {4}, {2}, {0}},
			deps: [][]int{{}, {}, {}, {1, 2}, {0, 3}},
		},
		{
			name: "cycle",
			graph: `.#...
					..#..
					...#.
					....#
					#....`,
			want: [][]int{{0, 1, 2, 3, 4}},
			deps: [][]int{{}},
		},
		{
			name: "two-cycles",
			graph: `.#...
					#..#.
					....#
					..#..
					...#.`,
			want: [][]int{{2, 3, 4}, {0, 1}},
			deps: [][]int{{}, {0}},
		},
		{
			name: "dumbbell",
			graph: `.#...
					#.#..
					..#.#
					....#
					...#.`,
			want: [][]int{{3, 4}, {2}, {0, 1}},
			deps: [][]int{{}, {0}, {1}},
		},
		{
			name: "cycle-tree",
			graph: `01234567
					.#...... 0
					#.#.#... 1
					...#.... 2
					..#...#. 3
					.....#.. 4
					....#... 5
					.......# 6
					......#. 7`,
			want: [][]int{{6, 7}, {2, 3}, {4, 5}, {0, 1}},
			deps: [][]int{{}, {0}, {}, {1, 2}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := parseGraph(tt.graph)
			dag := scc.Sort(0, g.deps)

			var got, gotDeps [][]int
			for c := range dag.Topological() {
				members := slices.Clone(c.Members())
				slices.Sort(members)
				got = append(got, members)

				deps := []int{}
				for c := range c.Deps() {
					deps = append(deps, c.Index())
				}
				slices.Sort(deps)
				gotDeps = append(gotDeps, deps)
			}

			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.deps, gotDeps)
		})
	}
}

// graph is a directed in matrix form. There is an edge from n to m if
// the value at matrix[nodes*n+m] is true.
type graph struct {
	nodes  int
	matrix []bool // len == nodes*nodes
}

// . means false, # means true. The total number of .s and #s must be.
func parseGraph(s string) graph {
	matrix := []bool{}
	for _, r := range s {
		switch r {
		case '.':
			matrix = append(matrix, false)
		case '#':
			matrix = append(matrix, true)
		}
	}

	// Check that len(entries) is a perfect square.
	nodes := int(math.Sqrt(float64(len(matrix))))
	if nodes*nodes != len(matrix) {
		panic("invalid graph string")
	}

	return graph{nodes, matrix}
}

// deps implements the scc.Graph interface.
func (g graph) deps(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for m := range g.nodes {
			idx := n*g.nodes + m
			if g.matrix[idx] && !yield(m) {
				return
			}
		}
	}
}
