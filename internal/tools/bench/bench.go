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

// bench is a script for running benchmarks and generating a
// pretty-printed report for putting into commit messages.
package main

import (
	"cmp"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// prefixes is a list of SI prefixes.
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

// common returns the common prefix length of a and b.
func common(a, b string) int {
	var i int
	for ; i < min(len(a), len(b)) && a[i] == b[i]; i++ {
	}
	return i
}

func main() {
	argv0, ok := os.LookupEnv("GO")
	if !ok {
		argv0 = "go"
	}

	stdout := new(strings.Builder)

	cmd := exec.Command(argv0, "test", "-run", "^B")
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
	type key struct {
		column string
		row    int
	}

	names := []string{}
	prettyNames := []string{}
	values := map[key]string{}
	order := map[string]int{}
	units := map[string]string{}
	var i int
	for _, line := range strings.Split(stdout.String(), "\n") {
		if !strings.HasPrefix(line, "Benchmark") {
			continue
		}
		// Split each benchmark into fields. Each field is separated by tabs.
		fields := strings.Split(line, "\t")
		fields = slices.Delete(fields, 1, 2) // Delete the trial count.
		for j := range fields {
			fields[j] = strings.TrimSpace(fields[j])
			if fields[j] == "" {
				continue
			}

			switch {
			case j == 0:
				name := fields[0]
				// Trim off a trailing -n, since it's not especially
				// interesting.
				name = name[:strings.LastIndex(fields[j], "-")]

				// Replace everything up to the first / with a .
				name = "." + name[strings.Index(fields[j], "/"):]

				// Delete all occurrences of .yaml.
				name = strings.ReplaceAll(name, ".yaml", "")
				names = append(names, name)

				// Replace the common prefix of this and the previous
				// benchmark with dashes.
				if len(names) > 1 {
					prev := names[len(names)-2]
					k := common(prev, name)
					k = strings.LastIndexByte(name[:k], '/')

					if k > 0 {
						bytes := []byte(name)
						for i, b := range bytes[1:k] {
							if b != '/' {
								bytes[i+1] = '\''
							}
						}
						name = string(bytes)
					}
				}

				prettyNames = append(prettyNames, name)

			case fields[j][0] <= 0 || fields[j][0] >= 9:
				num, unit, ok := strings.Cut(fields[j], " ")
				if !ok {
					continue
				}

				value, err := strconv.ParseFloat(num, 64)
				if err != nil {
					panic(err)
				}

				unit = strings.TrimSuffix(unit, "/op")
				column := unit
				switch unit {
				// Normalize some units.
				case "ns":
					column = "time"
					unit = "s"
					value *= 1e-9
				case "MB/s":
					column = "throughput"
					unit = "B/s"
					value *= 1e6
				case "B":
					column = "memory"
				case "allocs":
					column = "allocations"
				default:
					idx := strings.LastIndex(unit, "/")
					if idx > 0 {
						unit = unit[:idx]
					}
				}
				units[column] = unit

				// Pick the largest unit prefix smaller than field.units.
				if value == 0 {
					unit = " " + unit
				} else {
					for _, prefix := range prefixes {
						if prefix.mult <= value {
							value /= prefix.mult
							unit = prefix.prefix + unit
							break
						}
					}
				}

				cell := fmt.Sprintf("%.03f %v", value, unit)
				values[key{column, i}] = cell
				order[column] = max(j, order[column])
			}
		}

		i++
	}

	// Lay out the table.
	header := []string{""}
	for k := range order {
		header = append(header, k)
	}
	slices.SortStableFunc(header[1:], func(a, b string) int {
		x, y := order[a], order[b]
		if x != y {
			return x - y
		}
		return cmp.Compare(a, b)
	})

	table := [][]string{header}
	for i, name := range prettyNames {
		fields := []string{name}
		for _, k := range header[1:] {
			value := values[key{k, i}]
			if value == "" {
				value = "n/a  " + units[k]
			}
			fields = append(fields, value)
		}
		table = append(table, fields)
	}

	widths := make([]int, len(table[0]))
	for _, fields := range table {
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
	for _, fields := range table {
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
