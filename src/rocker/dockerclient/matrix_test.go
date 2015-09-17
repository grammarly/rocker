/*-
 * Copyright 2015 Grammarly, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dockerclient

import "testing"

func TestDockerIsInMatrix(t *testing.T) {
	result, err := IsInMatrix()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("is matrix: %v", result)
}

func TestDockerMyDockerId(t *testing.T) {
	id, err := MyDockerID()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("my docker id: %q", id)
}

func TestResolveHostPath(t *testing.T) {
	// we will need docker client to cleanup and do some cross-checks
	client, err := New()
	if err != nil {
		t.Fatal(err)
	}

	result, err := ResolveHostPath("/bin/rsync", client)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Result path: %s\n", result)
}
