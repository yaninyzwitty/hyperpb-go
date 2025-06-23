package testdata

import (
	"bytes"
	"embed"
	"encoding/hex"
	"io/fs"
	"path/filepath"
	"regexp"
	"runtime"
	runtimedebug "runtime/debug"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bufbuild/hyperpb"
	"github.com/bufbuild/hyperpb/internal/debug"
	"github.com/bufbuild/hyperpb/internal/prototest"
	"github.com/bufbuild/hyperpb/internal/tdp/compiler"
	"github.com/bufbuild/hyperpb/internal/tdp/profile"
	"github.com/protocolbuffers/protoscope"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"gopkg.in/yaml.v3"
)

//go:embed *
var testdata embed.FS

// Harness is a generalization of [testing.TB] that also includes the
// [testing.T.Run] method. It must be generic because the signature of this
// function varies across [testing.T] and [testing.B].
type Harness[T any] interface {
	testing.TB
	Run(string, func(T)) bool
}

// TestCase is a TestCase case from the TestCase data corpus.
type TestCase struct {
	Name string `yaml:"-"`

	TypeName string `yaml:"type"`
	Type     struct {
		Gencode protoreflect.MessageType
		Fast    *hyperpb.Type
	} `yaml:"-"`

	// If set, run this test as a benchmark.
	Benchmark bool `yaml:"benchmark"`

	PGO Profile `yaml:"pgo"`

	// Three ways to encode the test: hex, textproto, and protoscope
	Hex        []string `yaml:"hex"`
	TextProto  []string `yaml:"textproto"`
	Protoscope []string `yaml:"protoscope"`

	Specimens [][]byte `yaml:"-"`
}

// Profile is a profiling rule, which matches a field and applies the
// given profiling information to it.
type Profile []struct {
	Pattern *regexp.Regexp `yaml:"pattern"`
	Profile struct {
		DecodeProbability float64 `yaml:"parse"`
		ExpectedCount     int     `yaml:"expected_count"`
		AssumeUTF8        bool    `yaml:"assume_utf8"`
	} `yaml:"-,inline"`
}

// ForField implements [compiler.Profile].
func (p Profile) ForField(site profile.Site) profile.Field {
	for _, rule := range p {
		if rule.Pattern.MatchString(string(site.Field.FullName())) {
			return profile.Field(rule.Profile)
		}
	}
	return site.DefaultProfile()
}

// RunAll runs all of the test cases against the given harness.
func RunAll[T Harness[T]](t T, f func(T, *TestCase)) {
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

			test := parseTestCase(t, path, data)
			if test != nil {
				f(t, test)
			}
		})

		return nil
	})
	require.NoError(t, err)
}

// Run executes a single test case.
func (test *TestCase) Run(t *testing.T, ctx *hyperpb.Shared, verbose bool) {
	t.Helper()

	run := func(t *testing.T, specimen []byte) {
		t.Helper()

		runtimedebug.SetPanicOnFault(true)
		defer debug.WithTesting(t)()

		// Parse using the gencode.
		m1 := test.Type.Gencode.New().Interface()
		err1 := proto.Unmarshal(specimen, m1)

		// Parse using hyperpb.
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

// parseTestCase parses a single test case from the given data.
//
// This will call t.FailNow() if testing fails.
func parseTestCase(t testing.TB, path string, file []byte) *TestCase {
	t.Helper()
	defer debug.WithTesting(t)()

	require.True(t, bytes.HasSuffix(file, []byte("\n")), "missing trailing newline in %q", path)

	test := new(TestCase)
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

	test.Type.Fast = hyperpb.Compile(
		test.Type.Gencode.Descriptor(),
		hyperpb.WithExtensionsFromTypes(protoregistry.GlobalTypes),
		func(o *compiler.Options) { o.Profile = test.PGO },
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
