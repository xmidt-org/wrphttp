// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrphttp

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNegotiatedMediaType(t *testing.T) {
	tests := []struct {
		name        string
		ct          string
		accept      string
		headers     http.Header
		want        string
		expectError bool
	}{
		{
			name:   "Exact match JSON",
			accept: "application/wrp+json",
			want:   MEDIA_TYPE_JSON,
		},
		{
			name:   "Exact match Msgpack",
			accept: "application/wrp+msgpack",
			want:   MEDIA_TYPE_MSGPACK,
		},
		{
			name:   "Subtype wildcard picks MsgpackL",
			accept: "application/*",
			want:   MEDIA_TYPE_MSGPACKL,
		},

		{
			name:   "Wildcard */* picks MsgpackL",
			accept: "*/*",
			want:   MEDIA_TYPE_MSGPACKL,
		},
		{
			name:   "Multiple types prefer JSONL",
			accept: "application/wrp+jsonl;q=0.9, application/wrp+msgpack;q=0.8",
			want:   MEDIA_TYPE_JSONL,
		},
		{
			name:   "Multiple types prefer JSONL",
			accept: "application/wrp+json;q=0.2, application/wrp+jsonl;q=0.8",
			want:   MEDIA_TYPE_JSONL,
		},
		{
			name:   "No Accept header falls back to content type",
			accept: "",
			ct:     "application/wrp+json",
			want:   MEDIA_TYPE_JSON,
		},
		{
			name:   "No Accept header falls back to content type, and X-Webpa-Device-Name",
			accept: "",
			ct:     "application/wrp+octet-stream",
			want:   MEDIA_TYPE_OCTET_STREAM_WEBPA_STYLE,
			headers: http.Header{
				"X-Webpa-Device-Name": []string{"ignored"},
			},
		},
		{
			name:   "No Accept header falls back to content type, and X-Xmidt-Message-Type",
			accept: "",
			ct:     "application/wrp+octet-stream",
			want:   MEDIA_TYPE_OCTET_STREAM_WEBPA_STYLE,
			headers: http.Header{
				"X-Xmidt-Message-Type": []string{"ignored"},
			},
		},
		{
			name:   "No Accept header falls back to content type, and Xmidt-Message-Type",
			accept: "",
			ct:     "application/wrp+octet-stream",
			want:   MEDIA_TYPE_OCTET_STREAM_XMIDT_STYLE,
			headers: http.Header{
				"Xmidt-Message-Type": []string{"ignored"},
			},
		},
		{
			name:   "No Accept header falls back to content type, and X-Webpa-Device-Name",
			accept: "",
			ct:     "application/wrp+octet-stream",
			want:   MEDIA_TYPE_OCTET_STREAM_WEBPA_STYLE,
			headers: http.Header{
				"X-Webpa-Device-Name": []string{"ignored"},
			},
		},
		{
			name:   "Accept header with style parameter",
			accept: "application/wrp+octet-stream; style=x-xmidt",
			ct:     "application/wrp+msgpack",
			want:   MEDIA_TYPE_OCTET_STREAM_X_XMIDT_STYLE,
		},

		// Error cases

		{
			name:        "Unsupported type returns error",
			accept:      "image/jpeg",
			expectError: true,
		},
		{
			name:        "No Accept header falls back to content type, is invalid",
			accept:      "",
			ct:          "/wrp+json",
			expectError: true,
		},
		{
			name:        "Invalid Accept header",
			accept:      "/wrp+json",
			expectError: true,
		},
		{
			name:        "No Accept header falls back to content type, is invalid",
			accept:      "",
			ct:          "image/jpeg",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/", nil)

			if tt.headers != nil {
				req.Header = tt.headers
			}

			req.Header.Set("Accept", tt.accept)
			req.Header.Set("Content-Type", tt.ct)

			mt, err := NegotiateMediaType(req)

			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, mt)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, mt)
		})
	}
}
