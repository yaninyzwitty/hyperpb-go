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

package fastpb_test

import (
	"embed"
	"encoding/hex"
	"io/fs"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/protocolbuffers/protoscope"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	_ "google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"gopkg.in/yaml.v3"

	"github.com/bufbuild/fastpb"
	"github.com/bufbuild/fastpb/internal/dbg"
	_ "github.com/bufbuild/fastpb/internal/gen/test"
	"github.com/bufbuild/fastpb/internal/prototest"
)

func TestUnmarshal(t *testing.T) {
	t.Parallel()
	for _, test := range parseTests(t) {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			test.run(t, nil)
		})
	}
}

func BenchmarkUnmarshal(b *testing.B) {
	for _, test := range parseTests(b) {
		b.Run(test.Name, func(b *testing.B) {
			b.Run("fastpb", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(test.Bytes)))
				for range b.N {
					m := fastpb.New(test.Type.Fast)
					_ = proto.Unmarshal(test.Bytes, m)
				}
			})
			b.Run("amortize", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(test.Bytes)))
				ctx := new(fastpb.Context)
				for range b.N {
					m := ctx.New(test.Type.Fast)
					_ = proto.Unmarshal(test.Bytes, m)
					ctx.Free()
				}
			})
			b.Run("gencode", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(test.Bytes)))
				for range b.N {
					m := test.Type.Gencode.New().Interface()
					_ = proto.Unmarshal(test.Bytes, m)
				}
			})
			b.Run("dynamicpb", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(test.Bytes)))
				for range b.N {
					m := dynamicpb.NewMessage(test.Type.Gencode.Descriptor())
					_ = proto.Unmarshal(test.Bytes, m)
				}
			})
		})
	}
}

type test struct {
	Name string `yaml:"-"`

	TypeName string `yaml:"type"`
	Type     struct {
		Gencode protoreflect.MessageType
		Fast    fastpb.Type
	} `yaml:"-"`

	// If set, run this test as a benchmark.
	Benchmark bool `yaml:"benchmark"`

	Profile map[string]FieldProfile `yaml:"profile"`

	// Three ways to encode the test: hex, textproto, and protoscope.
	Hex        string `yaml:"hex"`
	TextProto  string `yaml:"textproto"`
	Protoscope string `yaml:"protoscope"`

	Bytes []byte `yaml:"-"`
}

// Copy of fastpb.FieldProfile with yaml annotations.
type FieldProfile struct {
	Cold bool `yaml:"cold"`
}

// Ensure that the above type matches the exported version.
var _ = fastpb.FieldProfile(FieldProfile{})

//go:embed testdata/*
var testdata embed.FS

func parseTests(t testing.TB) []*test {
	t.Helper()

	var tests []*test
	err := fs.WalkDir(testdata, ".", func(path string, d fs.DirEntry, err error) error {
		require.NoError(t, err, "loading test %q", path)

		if d.IsDir() || filepath.Ext(path) != ".yaml" {
			return nil
		}

		data, err := fs.ReadFile(testdata, path)
		require.NoError(t, err, "loading test %q", path)

		test := parseTest(t, path, data)
		if test != nil {
			tests = append(tests, test)
		}

		return nil
	})
	require.NoError(t, err)

	return tests
}

func parseTest(t testing.TB, path string, file []byte) *test {
	t.Helper()
	defer dbg.WithTesting(t)()

	test := new(test)
	err := yaml.Unmarshal(file, &test)
	require.NoError(t, err, "loading test %q", path)
	if _, bench := t.(*testing.B); bench && !test.Benchmark {
		return nil
	}

	test.Name = strings.TrimPrefix(path, "testdata/")
	test.Type.Gencode, err = protoregistry.GlobalTypes.FindMessageByName(
		protoreflect.FullName(test.TypeName))
	require.NoError(t, err, "loading type %q", test.TypeName)

	test.Type.Fast = fastpb.Compile(
		test.Type.Gencode.Descriptor(),
		fastpb.PGO(func(site fastpb.FieldSite) fastpb.FieldProfile {
			return fastpb.FieldProfile(test.Profile[string(site.Field.FullName())])
		}),
	)

	switch {
	case test.Hex != "":
		r := strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "")
		test.Bytes, err = hex.DecodeString(r.Replace(test.Hex))
		require.NoError(t, err, "loading test %q", path)

	case test.TextProto != "":
		m := test.Type.Gencode.New().Interface()
		err = prototext.Unmarshal([]byte(test.TextProto), m)
		require.NoError(t, err, "loading test %q", path)

		test.Bytes, err = proto.Marshal(m)
		require.NoError(t, err, "loading test %q", path)

	case test.Protoscope != "":
		s := protoscope.NewScanner(test.Protoscope)
		test.Bytes, err = s.Exec()
		require.NoError(t, err, "loading test %q", path)
	}

	return test
}

func (test *test) run(t testing.TB, ctx *fastpb.Context) {
	t.Helper()

	debug.SetPanicOnFault(true)
	defer dbg.WithTesting(t)()

	// Parse using the gencode.
	m1 := test.Type.Gencode.New().Interface()
	err1 := proto.Unmarshal(test.Bytes, m1)

	// Parse using fastpb.
	m2 := ctx.New(test.Type.Fast)
	err2 := proto.Unmarshal(test.Bytes, m2)

	if err1 != nil {
		require.Error(t, err2, "gencode error: %v", err1)
		return
	}

	require.NoError(t, err2)
	prototest.Equal(t, m1, m2)
}
