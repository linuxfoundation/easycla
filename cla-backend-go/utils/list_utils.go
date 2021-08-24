// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

// FindInt64Duplicates returns true if the two lists include any duplicates, false otherwise. Returns the duplicates
func FindInt64Duplicates(a, b []int64) []int64 {
	var duplicates []int64
	for i := 0; i < len(a); i++ {
		for j := 0; j < len(b); j++ {
			if a[i] == b[j] {
				duplicates = append(duplicates, a[i])
			}
		}
	}

	return duplicates
}
