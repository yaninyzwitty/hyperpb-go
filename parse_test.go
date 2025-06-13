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

package fastpb_test

import (
	"bytes"
	"embed"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/protocolbuffers/protoscope"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	_ "google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"gopkg.in/yaml.v3"

	"github.com/bufbuild/fastpb"
	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/flag2"
	_ "github.com/bufbuild/fastpb/internal/gen/test"
	"github.com/bufbuild/fastpb/internal/prototest"
)

var verbose bool

func TestMain(m *testing.M) {
	flag.Parse()
	verbose = flag2.Lookup[bool]("test.v")

	if flag2.Lookup[string]("test.bench") != "" {
		// Annoyingly, benchmarking won't print the compiler used...
		fmt.Printf("compiler: %v %v\n", runtime.Compiler, runtime.Version())
	}

	m.Run()
}

func TestUnmarshal(t *testing.T) {
	t.Parallel()
	runTests(t, func(t *testing.T, test *test) {
		t.Helper()
		test.run(t, nil)
	})
}

func BenchmarkUnmarshal(b *testing.B) {
	runTests(b, func(b *testing.B, test *test) {
		b.Helper()

		run := func(b *testing.B, specimen []byte) {
			b.Helper()
			b.Run("fastpb", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(specimen)))
				for range b.N {
					m := fastpb.New(test.Type.Fast)
					_ = proto.Unmarshal(specimen, m)
				}
			})
			b.Run("amortize", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(specimen)))
				ctx := new(fastpb.Shared)
				for range b.N {
					m := ctx.New(test.Type.Fast)
					_ = proto.Unmarshal(specimen, m)
					ctx.Free()
				}
			})
			b.Run("gencode", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(specimen)))
				for range b.N {
					m := test.Type.Gencode.New().Interface()
					_ = proto.Unmarshal(specimen, m)
				}
			})
			b.Run("dynamicpb", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(specimen)))
				for range b.N {
					m := dynamicpb.NewMessage(test.Type.Gencode.Descriptor())
					_ = proto.Unmarshal(specimen, m)
				}
			})
		}

		if len(test.Specimens) == 1 {
			run(b, test.Specimens[0])
			return
		}

		for _, specimen := range test.Specimens {
			b.Run("", func(b *testing.B) { run(b, specimen) })
		}
	})
}

type test struct {
	Name string `yaml:"-"`

	TypeName string `yaml:"type"`
	Type     struct {
		Gencode protoreflect.MessageType
		Fast    *fastpb.Type
	} `yaml:"-"`

	// If set, run this test as a benchmark.
	Benchmark bool `yaml:"benchmark"`

	PGO []struct {
		Pattern *regexp.Regexp `yaml:"pattern"`
		Profile struct {
			Parse fastpb.Temperature `yaml:"parse"`
		} `yaml:"-,inline"`
	} `yaml:"pgo"`

	// Three ways to encode the test: hex, textproto, and protoscope
	Hex        []string `yaml:"hex"`
	TextProto  []string `yaml:"textproto"`
	Protoscope []string `yaml:"protoscope"`

	Specimens [][]byte `yaml:"-"`
}

//go:embed testdata/*
var testdata embed.FS

type testingT[T any] interface {
	testing.TB
	Run(string, func(T)) bool
}

func runTests[T testingT[T]](t T, f func(T, *test)) {
	t.Helper()

	var failed atomic.Bool
	err := fs.WalkDir(testdata, ".", func(path string, d fs.DirEntry, err error) error {
		require.NoError(t, err, "loading test %q", path)

		if d.IsDir() || filepath.Ext(path) != ".yaml" {
			return nil
		}

		t.Run(strings.TrimPrefix(path, "testdata/"), func(t T) {
			if t, ok := any(t).(*testing.T); ok {
				t.Parallel()
			}

			defer failed.CompareAndSwap(false, t.Failed())

			data, err := fs.ReadFile(testdata, path)
			require.NoError(t, err, "loading test %q", path)

			test := parseTest(t, path, data)
			if test != nil {
				f(t, test)
			}
		})

		return nil
	})
	require.NoError(t, err)
}

func parseTest(t testing.TB, path string, file []byte) *test {
	t.Helper()
	defer dbg.WithTesting(t)()

	require.True(t, bytes.HasSuffix(file, []byte("\n")), "missing trailing newline in %q", path)

	test := new(test)
	dec := yaml.NewDecoder(bytes.NewReader(file))
	dec.KnownFields(true)
	err := dec.Decode(&test)
	require.NoError(t, err, "loading test %q", path)
	if b, ok := t.(*testing.B); ok && !test.Benchmark {
		b.SkipNow()
	}

	test.Name = strings.TrimPrefix(path, "testdata/")
	test.Type.Gencode, err = protoregistry.GlobalTypes.FindMessageByName(
		protoreflect.FullName(test.TypeName))
	require.NoError(t, err, "loading type %q", test.TypeName)

	test.Type.Fast = fastpb.Compile(
		test.Type.Gencode.Descriptor(),
		fastpb.PGO(func(site fastpb.FieldSite) fastpb.FieldProfile {
			for _, rule := range test.PGO {
				if rule.Pattern.MatchString(string(site.Field.FullName())) {
					return fastpb.FieldProfile(rule.Profile)
				}
			}
			return fastpb.FieldProfile{}
		}),
		fastpb.ExtensionsFromRegistry(protoregistry.GlobalTypes),
	)

	for _, raw := range test.Hex {
		r := strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "")
		b, err := hex.DecodeString(r.Replace(raw))
		require.NoError(t, err, "loading test %q", path)

		test.Specimens = append(test.Specimens, b)
	}

	for _, raw := range test.TextProto {
		m := test.Type.Gencode.New().Interface()
		err = prototext.Unmarshal([]byte(raw), m)
		require.NoError(t, err, "loading test %q", path)

		b, err := proto.Marshal(m)
		require.NoError(t, err, "loading test %q", path)

		test.Specimens = append(test.Specimens, b)
	}

	for _, raw := range test.Protoscope {
		s := protoscope.NewScanner(raw)
		b, err := s.Exec()
		require.NoError(t, err, "loading test %q", path)

		test.Specimens = append(test.Specimens, b)
	}

	return test
}

func (test *test) run(t *testing.T, ctx *fastpb.Shared) {
	t.Helper()

	run := func(t *testing.T, specimen []byte) {
		t.Helper()

		debug.SetPanicOnFault(true)
		defer dbg.WithTesting(t)()

		// Parse using the gencode.
		m1 := test.Type.Gencode.New().Interface()
		err1 := proto.Unmarshal(specimen, m1)

		// Parse using fastpb.
		m2 := ctx.New(test.Type.Fast)
		err2 := proto.Unmarshal(specimen, m2)

		if verbose {
			t.Logf("theirs: %v, ours: %v", err1, err2)
		}

		if err1 != nil {
			require.Error(t, err2, "gencode error: %v", err1)
			return
		}
		require.NoError(t, err2)

		runtime.GC()
		prototest.Equal(t, m1, m2)

		if verbose {
			options := protojson.MarshalOptions{
				Multiline:     true,
				Indent:        "  ",
				UseProtoNames: true,
			}
			b1, _ := options.Marshal(m1)
			b2, _ := options.Marshal(m2)
			t.Logf("theirs: %s", b1)
			t.Logf("ours: %s", b2)
		}
	}

	if len(test.Specimens) == 1 {
		run(t, test.Specimens[0])
		return
	}

	for _, specimen := range test.Specimens {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			run(t, specimen)
		})
	}
}
