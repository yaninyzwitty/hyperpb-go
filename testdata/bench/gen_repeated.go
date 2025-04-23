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

// gen_repeated is a script for generating blocks of repeated Protoscope fields.
package main

import (
	"flag"
	"fmt"
	"math/bits"
	"math/rand/v2"
)

var (
	lo  = flag.Uint64("lo", 0, "lower bound (inclusive)")
	hi  = flag.Uint64("hi", 0, "upper bound (exclusive)")
	n   = flag.Int("n", 0, "the number of elements to generate")
	row = flag.Int("row", 16, "the number of elements to a row")

	format = flag.String("f", "", "the format to print each value in")
	zipf   = flag.Bool("zipf", false, "use a Zipf distribution rather than a uniform one")
)

func main() {
	flag.Parse()

	var cells [][]string
	for i := range *n {
		if i%*row == 0 {
			cells = append(cells, nil)
		}

		v := rand.Uint64N(*hi-*lo) + *lo
		if *zipf {
			// We don't bother with rand.Zipf. Instead, we pick a random bit
			// length between 0 and the bit length of hi and truncate v to that.
			k := bits.Len64(*hi)
			k = rand.IntN(k) + 1
			v &= (uint64(1) << k) - 1
		}

		cells[len(cells)-1] = append(cells[len(cells)-1], fmt.Sprintf(*format, v))
	}

	// Discover the widest cell in each column.
	var maxima []int
	for _, row := range cells {
		for col, cell := range row {
			if len(maxima) <= col {
				maxima = append(maxima, 0)
			}

			maxima[col] = max(maxima[col], len(cell))
		}
	}

	// Snap each maximum to an even number.
	for i, n := range maxima {
		maxima[i] = (n + 2) &^ 1
	}

	maxima[len(maxima)-1] = 0 // No need to pad the final cell.

	// Render each row with the appropriate padding between them.
	for _, row := range cells {
		for col, cell := range row {
			fmt.Printf("%-*s", maxima[col], cell)
		}
		fmt.Println()
	}
}
