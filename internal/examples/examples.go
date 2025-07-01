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

// Package examples contains helpers for making the examples in
// hyperpb/example_test.go easier to read.
package examples

//go:generate go run ../tools/genexamples

//hyperpb:example DownloadWeatherReportSchema
// // DownloadWeatherReportSchema returns a compiled Protobuf schema for a
// // weather report protocol.
//
// 1: {
//   1: {"internal/proto/example/weather/v1/weather.proto"}
//   2: {"example.weather.v1"}
//   4: {
//     1: {"StationReport"}
//     2: {
//       1: {"station"}
//       3: 1
//       4: 1
//       5: 9
//       10: {"station"}
//     }
//     2: {
//       1: {"frequency"}
//       3: 2
//       4: 1
//       5: 2
//       10: {"frequency"}
//     }
//     2: {
//       1: {"temperature"}
//       3: 3
//       4: 1
//       5: 2
//       10: {"temperature"}
//     }
//     2: {
//       1: {"pressure"}
//       3: 4
//       4: 1
//       5: 2
//       10: {"pressure"}
//     }
//     2: {
//       1: {"wind_speed"}
//       3: 5
//       4: 1
//       5: 2
//       10: {"windSpeed"}
//     }
//     2: {
//       1: {"conditions"}
//       3: 6
//       4: 1
//       5: 14
//       6: {".example.weather.v1.Condition"}
//       10: {"conditions"}
//     }
//   }
//   4: {
//     1: {"WeatherReport"}
//     2: {
//       1: {"region"}
//       3: 1
//       4: 1
//       5: 9
//       10: {"region"}
//     }
//     2: {
//       1: {"weather_stations"}
//       3: 2
//       4: 3
//       5: 11
//       6: {".example.weather.v1.StationReport"}
//       10: {"weatherStations"}
//     }
//   }
//   5: {
//     1: {"Condition"}
//     2: {
//       1: {"CONDITION_UNSPECIFIED"}
//       2: 0
//     }
//     2: {
//       1: {"CONDITION_SUNNY"}
//       2: 1
//     }
//     2: {
//       1: {"CONDITION_RAINY"}
//       2: 2
//     }
//     2: {
//       1: {"CONDITION_OVERCAST"}
//       2: 3
//     }
//   }
// 	 12: {"proto3"}
// }

//hyperpb:example ReadWeatherData
// // ReadWeatherData downloads some weather report data from the internet,
// // using the protocol from [DownloadWeatherReportSchema].
//
// 1: {"Seattle"}
// 2: {
//   1: {"KAD93"}
//   2: 162.525i32
//   3: 11.3i32
//   4: 30.08i32
//   5: 2.3i32
//   6: 3
// }
// 2: {
//   1: {"KHB60"}
//   2: 162.55i32
//   3: 13.7i32
//   4: 28.09i32
//   5: 1.9i32
//   6: 3
// }
