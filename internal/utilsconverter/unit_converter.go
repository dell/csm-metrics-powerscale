/*
 Copyright (c) 2022 Dell Inc. or its subsidiaries. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package utilsconverter

// DiskAllocUnits presents the unit for disk allocation.
type DiskAllocUnits uint64

const prefixScale = 1024

const (
	// BITS bit unit
	BITS DiskAllocUnits = 1
	// BYTES byte unit
	BYTES DiskAllocUnits = 8
	// KB kilobyte unit
	KB DiskAllocUnits = prefixScale * BYTES
	// MB megabyte unit
	MB DiskAllocUnits = prefixScale * KB
	// GB gigabyte unit
	GB DiskAllocUnits = prefixScale * MB
	// TB terabyte unit
	TB DiskAllocUnits = prefixScale * GB
)

// Number presents numeric value.
type Number interface {
	int | int8 | int32 | int64 | float64
}

// UnitsConvert converts a value based on the given units.
func UnitsConvert[N Number](value N, from DiskAllocUnits, to DiskAllocUnits) float64 {
	return float64(value) * float64(from) / float64(to)
}
