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

package examples

import "encoding/base64"

// Helpers for making the examples in `./example_test.go` easier to read.

func DownloadWeatherReportSchema() []byte {
	bytes, err := base64.StdEncoding.DecodeString(`CpQECi9pbnRlcm5hbC9wcm90by9leGFtcGxlL3dlYXRoZXIvdjEvd2VhdGhlci5wcm90bxISZXhhbXBsZS53ZWF0aGVyLnYxIuMBCg1TdGF0aW9uUmVwb3J0EhgKB3N0YXRpb24YASABKAlSB3N0YXRpb24SHAoJZnJlcXVlbmN5GAIgASgCUglmcmVxdWVuY3kSIAoLdGVtcGVyYXR1cmUYAyABKAJSC3RlbXBlcmF0dXJlEhoKCHByZXNzdXJlGAQgASgCUghwcmVzc3VyZRIdCgp3aW5kX3NwZWVkGAUgASgCUgl3aW5kU3BlZWQSPQoKY29uZGl0aW9ucxgGIAEoDjIdLmV4YW1wbGUud2VhdGhlci52MS5Db25kaXRpb25SCmNvbmRpdGlvbnMidQoNV2VhdGhlclJlcG9ydBIWCgZyZWdpb24YASABKAlSBnJlZ2lvbhJMChB3ZWF0aGVyX3N0YXRpb25zGAIgAygLMiEuZXhhbXBsZS53ZWF0aGVyLnYxLlN0YXRpb25SZXBvcnRSD3dlYXRoZXJTdGF0aW9ucypoCglDb25kaXRpb24SGQoVQ09ORElUSU9OX1VOU1BFQ0lGSUVEEAASEwoPQ09ORElUSU9OX1NVTk5ZEAESEwoPQ09ORElUSU9OX1JBSU5ZEAISFgoSQ09ORElUSU9OX09WRVJDQVNUEANiBnByb3RvMw==`)
	if err != nil {
		panic(err)
	}
	return bytes
}

func ReadWeatherData() []byte {
	bytes, err := base64.StdEncoding.DecodeString(`CgdTZWF0dGxlEh0KBUtBRDkzFWaGIkMdzcw0QSXXo/BBLTMzE0AwAxIdCgVLSEI2MBXNjCJDHTMzW0ElUrjgQS0zM/M/MAM=`)
	if err != nil {
		panic(err)
	}
	return bytes
}
