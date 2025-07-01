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

package xprotoreflect_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/bufbuild/hyperpb"
	"github.com/bufbuild/hyperpb/internal/tdp/empty"
	"github.com/bufbuild/hyperpb/internal/xprotoreflect"
)

func TestScalar(t *testing.T) {
	t.Parallel()

	testScalar[int32](t)
	testScalar[uint32](t)
	testScalar[int64](t)
	testScalar[uint64](t)
	testScalar[protoreflect.EnumNumber](t)
}

func TestMessage(t *testing.T) {
	t.Parallel()

	ty := hyperpb.Compile[*emptypb.Empty]()
	m := ty.New()

	v := protoreflect.ValueOf(m)
	assert.Same(t, m, xprotoreflect.GetMessage[*hyperpb.Message](v))
	assert.Same(t, m, xprotoreflect.GetMessage[protoreflect.Message](v))
	assert.Panics(t, func() {
		w := protoreflect.ValueOf(nil)
		_ = xprotoreflect.GetMessage[*hyperpb.Message](w)
	})
	assert.Panics(t, func() {
		w := protoreflect.ValueOf(int32(42))
		_ = xprotoreflect.GetMessage[*hyperpb.Message](w)
	})
	assert.Panics(t, func() {
		w := protoreflect.ValueOf(empty.Message{})
		_ = xprotoreflect.GetMessage[*hyperpb.Message](w)
	})
}

func testScalar[T xprotoreflect.Int](t *testing.T) {
	t.Helper()

	var bits uint64 = 0xcdcdcdcdcdcdcdcd
	v := T(bits)
	t.Run(fmt.Sprintf("%T", v), func(t *testing.T) {
		t.Parallel()

		w := protoreflect.ValueOf(v)
		assert.Equal(t, v, xprotoreflect.GetInt[T](w))

		assert.Panics(t, func() {
			w := protoreflect.ValueOf(nil)
			_ = xprotoreflect.GetInt[T](w)
		})

		assert.Panics(t, func() {
			w := protoreflect.ValueOf(false)
			_ = xprotoreflect.GetInt[T](w)
		})
	})
}
