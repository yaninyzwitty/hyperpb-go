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
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/bufbuild/fastpb"
	testpb "github.com/bufbuild/fastpb/internal/gen/test"
	"github.com/bufbuild/fastpb/internal/sync2"
)

var contexts = sync2.Pool[fastpb.Context]{
	Reset: (*fastpb.Context).Free,
}

func FuzzScalars(f *testing.F) {
	fuzz[*testpb.Scalars](f)
}

func fuzz[M proto.Message](f *testing.F) {
	f.Helper()

	var z M
	test := new(test)
	test.Type.Gencode = z.ProtoReflect().Type()
	test.Type.Fast = fastpb.Compile(test.Type.Gencode.Descriptor())

	f.Fuzz(func(t *testing.T, b []byte) {
		ctx, drop := contexts.Get()
		defer drop()

		test := *test
		test.Bytes = b
		test.run(t, ctx)
	})
}
