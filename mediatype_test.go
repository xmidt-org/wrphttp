// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrphttp

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllMediaTypes(t *testing.T) {
	tests := []struct {
		name     string
		expected []string
	}{
		{
			name: "all media types",
			expected: []string{
				MEDIA_TYPE_JSON,
				MEDIA_TYPE_MSGPACK,
				MEDIA_TYPE_OCTET_STREAM,
				MEDIA_TYPE_JSONL,
				MEDIA_TYPE_MSGPACKL,
				MEDIA_TYPE_OCTET_STREAM_X_XMIDT_STYLE,
				MEDIA_TYPE_OCTET_STREAM_X_MIDT_STYLE,
				MEDIA_TYPE_OCTET_STREAM_XMIDT_STYLE,
				MEDIA_TYPE_OCTET_STREAM_WEBPA_STYLE,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := AllMediaTypes()

			sort.Strings(actual)
			sort.Strings(test.expected)

			assert.ElementsMatch(t, test.expected, actual)
		})
	}
}
