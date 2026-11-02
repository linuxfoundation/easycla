// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"
)

func TestPageBounds(t *testing.T) {
	cases := []struct {
		name      string
		total     int
		pageSize  *int64
		offset    *int64
		wantStart int
		wantEnd   int
	}{
		{"no paging", 10, nil, nil, 0, 10},
		{"page size only", 10, aws.Int64(3), nil, 0, 3},
		{"offset only", 10, nil, aws.Int64(4), 4, 10},
		{"page size and offset", 10, aws.Int64(3), aws.Int64(4), 4, 7},
		{"last partial page", 10, aws.Int64(3), aws.Int64(9), 9, 10},
		{"offset at end", 10, aws.Int64(3), aws.Int64(10), 10, 10},
		{"offset beyond end", 10, aws.Int64(3), aws.Int64(42), 10, 10},
		{"page size beyond end", 10, aws.Int64(100), nil, 0, 10},
		{"zero page size ignored", 10, aws.Int64(0), nil, 0, 10},
		{"zero offset ignored", 10, nil, aws.Int64(0), 0, 10},
		{"empty list", 0, aws.Int64(5), aws.Int64(2), 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := PageBounds(tc.total, tc.pageSize, tc.offset)
			assert.Equal(t, tc.wantStart, start, "start")
			assert.Equal(t, tc.wantEnd, end, "end")
		})
	}
}
