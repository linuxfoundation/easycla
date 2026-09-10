// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

// PageBounds converts the optional pageSize/offset query parameters into
// [start, end) slice bounds for an in-memory list of totalCount entries.
// When both parameters are nil the full range is returned, keeping the
// non-paged behavior unchanged.
func PageBounds(totalCount int, pageSize, offset *int64) (int, int) {
	start, end := 0, totalCount
	if offset != nil && *offset > 0 {
		if *offset >= int64(totalCount) {
			start = totalCount
		} else {
			start = int(*offset)
		}
	}
	if pageSize != nil && *pageSize > 0 && int64(end-start) > *pageSize {
		end = start + int(*pageSize)
	}
	return start, end
}
