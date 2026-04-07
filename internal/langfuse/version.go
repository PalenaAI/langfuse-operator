/*
Copyright 2026.

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

package langfuse

import "strings"

// NormalizeVersion strips the "v" prefix and returns a comparable version string.
func NormalizeVersion(tag string) string {
	return strings.TrimPrefix(tag, "v")
}

// VersionChanged returns true if the desired version differs from the current version.
func VersionChanged(desired, current string) bool {
	return NormalizeVersion(desired) != NormalizeVersion(current)
}
