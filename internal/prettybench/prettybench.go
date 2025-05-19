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

// prettybench is a script for running benchmarks and generating a
// pretty-printed report for putting into commit messages.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

var prefixes = []struct {
	prefix string
	mult   float64
}{
	{"E", 1e18},
	{"P", 1e15},
	{"T", 1e12},
	{"G", 1e9},
	{"M", 1e6},
	{"k", 1e3},
	{" ", 1e0},
	{"m", 1e-3},
	{"μ", 1e-6},
	{"n", 1e-9},
	{"p", 1e-12},
}

func main() {
	argv0, ok := os.LookupEnv("GO_CMD")
	if !ok {
		argv0 = "go"
	}

	stdout := new(strings.Builder)

	cmd := exec.Command(argv0, "test", "-run", "^B", "-benchmem")
	cmd.Args = append(cmd.Args, os.Args[1:]...)
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = io.MultiWriter(os.Stdout, stdout)
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if exit, ok := err.(*exec.ExitError); ok { //nolint:errorlint
		os.Exit(exit.ExitCode())
	} else if err != nil {
		panic(err)
	}

	fmt.Print("generating report...\n\n")

	// Extract all lines from stdout that begin with Benchmark. These are
	// benchmark results.
	names := []string{""}
	benchmarks := [][]string{{"", "time", "thoughput", "memory", "allocations"}}
	for _, line := range strings.Split(stdout.String(), "\n") {
		if !strings.HasPrefix(line, "Benchmark") {
			continue
		}
		// Split each benchmark into fields. Each field is separated by tabs.
		fields := strings.Split(line, "\t")
		fields = slices.Delete(fields, 1, 2) // Delete the trial count.
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
			if fields[i] == "" {
				continue
			}

			switch {
			case i == 0:
				// Trim off a trailing -n, since it's not especially
				// interesting.
				fields[i] = fields[i][:strings.LastIndex(fields[i], "-")]

				// Replace everything up to the first / with a .
				fields[i] = "." + fields[i][strings.Index(fields[i], "/"):] //nolint:gocritic

				// Delete all occurrences of .yaml.
				fields[i] = strings.ReplaceAll(fields[i], ".yaml", "")

				// Replace the common prefix of this and the previous
				// benchmark with dashes.
				names = append(names, fields[0])
				prev := names[len(benchmarks)-1]
				if prev != "" && fields[i] != "" {
					var k int
					for ; prev[k] == fields[i][k]; k++ {
					}
					if k > 0 && fields[i][k-1] == '/' {
						k--
					}
					if k >= 4 {
						fields[i] = " '' " + strings.Repeat(" ", k-4) + fields[i][k:]
					}
				}

			case fields[i][0] <= 0 || fields[i][0] >= 9:
				num, units, ok := strings.Cut(fields[i], " ")
				if !ok {
					continue
				}

				value, err := strconv.ParseFloat(num, 64)
				if err != nil {
					panic(err)
				}

				units = strings.TrimSuffix(units, "/op")
				switch units {
				// Normalize some units.
				case "ns":
					units = "s"
					value *= 1e-9
				case "MB/s":
					units = "B/s"
					value *= 1e6
				case "allocs":
				}

				// Pick the largest unit prefix smaller than field.units.
				for _, prefix := range prefixes {
					if prefix.mult < value {
						value /= prefix.mult
						units = prefix.prefix + units
						break
					}
				}

				fields[i] = fmt.Sprintf("%.03f %v", value, units)
			}
		}
		benchmarks = append(benchmarks, fields)
	}

	// Lay out the table.
	widths := make([]int, len(benchmarks[0]))
	for _, fields := range benchmarks {
		for i, field := range fields {
			widths[i] = max(widths[i], utf8.RuneCountInString(field))
		}
	}

	// Round all the widths up to a multiple of 2.
	for i := range widths {
		widths[i]++
		widths[i] &^= 1
	}

	// Print the table.
	for _, fields := range benchmarks {
		for i, field := range fields {
			if i == 0 {
				fmt.Printf("%s", field)
				fmt.Printf("%*s", widths[i]-utf8.RuneCountInString(field), "")
			} else {
				fmt.Printf(" | %+*s", widths[i], field)
			}
		}
		fmt.Println()
	}
	fmt.Println()
}
