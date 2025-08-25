/*
 *
 * Copyright © 2021-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

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

import (
	"testing"
)

func Test_UnitsConvert(t *testing.T) {
	got := UnitsConvert(1024, BYTES, KB)
	if got != 1 {
		t.Errorf("Float64UnitsConvert() = %f, expect 1", got)
	}

	got = UnitsConvert(1, MB, BYTES)
	if got != 1024*1024 {
		t.Errorf("Float64UnitsConvert() = %f, expect %d", got, 1024*1024)
	}

	got = UnitsConvert(1.5, GB, MB)
	if got != 1.5*1024 {
		t.Errorf("Float64UnitsConvert() = %f, expect %f", got, 1.5*1024)
	}

	got = UnitsConvert(9029296.123, MB, GB)
	if got != 9029296.123/1024 {
		t.Errorf("Float64UnitsConvert() = %f, expect %f", got, 9029296.123/1024)
	}
}
