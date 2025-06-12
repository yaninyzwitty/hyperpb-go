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

// prettyasm cleans up the output of go tool objdump into something readable.
//
// Based on the Go integration in Godbolt. See
// https://github.com/compiler-explorer/compiler-explorer/blob/61d10c320af1e3af1c39d776f6a0cd8868ca418b/lib/compilers/golang.ts#L57.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	jump = regexp.MustCompile(`^(J|B|CB|TB|CMPB|CMPUB).*`)
	text = regexp.MustCompile(`^TEXT\s+(.+?)\(SB\)\s*(.+)`)
	inst = regexp.MustCompile(`^\s*(.*?:-?\d+)\s+(0x[\da-f]+)\s+([\da-f]+)\s*([\w\.?]+)(.*)`)

	pcrel = regexp.MustCompile(`(-?\d+)\(PC\)$`)
	hex   = regexp.MustCompile(`(0x[\da-f]+)$`)
)

var (
	symbolPrefix = flag.String("prefix", "", "prefix to strip from symbols")
	info         = flag.String("info", "", "set to 'fileline' for line info and 'gnu' for GCC syntax")
	nops         = flag.Bool("nops", false, "if set, no-ops won't be filtered out")
)

type Func struct {
	Name string
	File string
	Code []Inst
}

type Inst struct {
	Loc      string
	PC       uint64
	Hex      string
	Mnemonic string
	Args     []string
	GNU      string

	Label string
}

// parseObjdump parses the output of go tool objdump.
func parseObjdump(data string) (fns []Func, err error) {
	prefix := *symbolPrefix

	symCleanup := func(arg string) string {
		arg = strings.TrimSpace(arg)
		arg = strings.ReplaceAll(arg, prefix+"/internal/", ".")
		arg = strings.ReplaceAll(arg, prefix, "")

		// Convert the generic prefix into as ~ for compactness.
		arg = strings.ReplaceAll(arg, "go.shape.", "~")

		return arg
	}

	for _, line := range strings.Split(data, "\n") {
		if line == "" {
			continue
		}

		if match := text.FindStringSubmatch(line); match != nil {
			fns = append(fns, Func{
				Name: symCleanup(match[1]),
				File: match[2],
			})
			continue
		}

		match := inst.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("invalid line: %s", line)
		}

		var inst Inst

		inst.Loc = match[1]
		inst.PC, _ = strconv.ParseUint(match[2], 0, 64)
		inst.Hex = match[3]
		inst.Mnemonic = match[4]
		args, gnu, _ := strings.Cut(match[5], "//")
		inst.Args = strings.Split(args, ",")
		inst.GNU = strings.TrimSpace(strings.TrimPrefix(gnu, "//"))

		for i := range inst.Args {
			inst.Args[i] = symCleanup(inst.Args[i])
		}

		if !*nops {
			switch inst.Mnemonic {
			case "NOP", "NOPW", "NOPL", "NOPQ", "NOOP", "?", "INT":
				continue
			}
		}

		cur := &fns[len(fns)-1]
		cur.Code = append(cur.Code, inst)
	}

	return fns, err
}

func run() error {
	// Read all of stdin.
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}

	fns, err := parseObjdump(string(data))
	if err != nil {
		return err
	}

	// Annotate each function with jump labels.
	type j struct {
		Target uint64
		Insts  []*Inst
	}
	jmap := make(map[uint64][]*Inst)
	var jumps []j
	for _, fn := range fns {
		clear(jmap)
		jumps = jumps[:0]
		for i := range fn.Code {
			inst := &fn.Code[i]
			if !jump.MatchString(inst.Mnemonic) {
				continue
			}

			if len(inst.Args) == 0 {
				continue
			}
			arg := inst.Args[len(inst.Args)-1]

			// Calculate the offset this jump is jumping to. We only really
			// support either direct jumps (like x86) or indirect jumps
			// (like aarch64).
			var target uint64
			if match := pcrel.FindStringSubmatch(arg); match != nil {
				displacement, _ := strconv.Atoi(match[1])
				target = inst.PC + uint64(displacement)*4
			} else if match := hex.FindStringSubmatch(arg); match != nil {
				target, _ = strconv.ParseUint(match[1], 0, 64)
			}

			// Delete the last argument from the instruction.
			inst.Args = inst.Args[:len(inst.Args)-1]

			// Record the instruction so we can generate a label for it.
			jmap[target] = append(jmap[target], inst)
		}

		name, _, _ := strings.Cut(fn.Name, "[")
		name = name[strings.LastIndex(name, "."):]

		// Now, annotate each instruction with the appropriate jump targets.
		// We want to iterate jumps in order so that the labels are assigned
		// consecutive values.
		idx := 1
		for i := range fn.Code {
			inst := &fn.Code[i]

			jumps := jmap[inst.PC]
			if jumps == nil {
				continue
			}

			inst.Label = fmt.Sprintf("%s.L%d", name, idx)
			idx++
			for _, jump := range jumps {
				jump.Args = append(jump.Args, inst.Label)
			}
		}
	}

	fmt.Print("//go:build disable\n\n")

	var fileLine, gnu bool
	switch *info {
	case "":
	case "fileline":
		fileLine = true
	case "gnu":
		gnu = true
	default:
		return fmt.Errorf("invalid value for -info: %v", *info)
	}

	// Pretty-print each function.
	line := new(bytes.Buffer)
	for _, fn := range fns {
		fmt.Printf("TEXT %s(SB)\n", fn.Name)

		// Find the widest mnemonic and arg string.
		var w1, w2 int
		for _, inst := range fn.Code {
			w1 = max(w1, len(inst.Mnemonic))
			w2 = max(w2, len(strings.Join(inst.Args, ", ")))
		}
		// Round both to a multiple of 2.
		w1 = (w1 + 1) &^ 1
		w2 = (w2 + 1) &^ 1

		// Clamp both to a reasonable maximum.
		w1 = min(w1, 16)
		w2 = min(w2, 40)

		prev := ""
		for _, inst := range fn.Code {
			line.Reset()
			if inst.Label != "" {
				fmt.Fprintf(line, "%s:\n", inst.Label)
			}

			fmt.Fprintf(line, "  %-*s  %-*s", w1, inst.Mnemonic, w2, strings.Join(inst.Args, ", "))

			switch {
			case fileLine:
				// Don't bother with line numbers for assembly functions.
				if fileLine && !strings.HasSuffix(fn.File, ".s") {
					if inst.Loc != prev {
						fmt.Fprintf(line, "  ; %v", inst.Loc)
						prev = inst.Loc
					}
				}
			case gnu:
				fmt.Fprintf(line, "  ; %v", inst.GNU)
			}

			fmt.Printf("%s\n", bytes.TrimRight(line.Bytes(), " "))
		}
		fmt.Println()
	}

	return nil
}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
