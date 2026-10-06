// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrphttp

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/wrp-go/v5"
)

func TestNewEncoder(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		err  bool
	}{
		{
			name: "default options",
			err:  false,
		},
		{
			name: "invalid MediaType",
			opts: []Option{
				AsMediaType("invalid"),
			},
			err: true,
		},
		{
			name: "empty MediaType",
			opts: []Option{
				AsMediaType(""),
			},
			err: true,
		},
		{
			name: "negotiate media type, msgpack",
			opts: []Option{
				AsNegotiated(&http.Request{
					Header: http.Header{
						"Accept": []string{
							"application/msgpack",
						},
					},
				}),
			},
			err: false,
		},
		{
			name: "negotiate media type, invalid",
			opts: []Option{
				AsNegotiated(&http.Request{
					Header: http.Header{
						"Accept": []string{
							"invalid",
						},
					},
				}),
			},
			err: true,
		},
		{
			name: "invalid OctetStream",
			opts: []Option{
				AsOctetStream("invalid"),
			},
			err: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoder, err := NewEncoder(test.opts...)
			if test.err {
				require.Error(t, err)
				assert.Nil(t, encoder)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, encoder)
		})
	}
}

func TestNewRequestWithContext(t *testing.T) {
	tests := []struct {
		name   string
		ctx    context.Context
		method string
		url    string
		msgs   []wrp.Union
		err    bool
	}{
		{
			name:   "invalid number of messages",
			ctx:    context.Background(),
			method: "POST",
			url:    "http://example.com",
			msgs:   []wrp.Union{},
			err:    true,
		},
		{
			name:   "invalid context",
			ctx:    nil,
			method: "POST",
			url:    "http://example.com",
			msgs: []wrp.Union{
				&wrp.Message{
					Source:      "source",
					Destination: "destination",
				},
			},
			err: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoder, err := NewEncoder(EncodeValidators(wrp.NoStandardValidation()))
			require.NoError(t, err)
			require.NotNil(t, encoder)
			req, err := encoder.NewRequestWithContext(test.ctx, test.method, test.url, test.msgs...)
			if test.err {
				require.Error(t, err)
				assert.Nil(t, req)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, req)
		})
	}
}

func TestAsParts(t *testing.T) {
	tests := []struct {
		name    string
		msgs    []wrp.Union
		opts    []Option
		err     bool
		readErr bool
	}{
		{
			name: "invalid message that fails validation",
			msgs: []wrp.Union{
				&wrp.Message{
					Source:      "source",
					Destination: "destination",
				},
			},
			opts: []Option{
				AsOctetStream(),
			},
			err: true,
		},
		{
			name: "invalid msgpack message that fails during read",
			msgs: []wrp.Union{
				&wrp.Message{
					Source:      "source",
					Destination: "destination",
				},
			},
			readErr: true,
		},
		{
			name: "invalid msgpack messages that fails during read",
			msgs: []wrp.Union{
				&wrp.Message{
					Source:      "source",
					Destination: "destination",
				},
				&wrp.Message{
					Source:      "source",
					Destination: "destination",
				},
			},
			readErr: true,
		},
		{
			name: "invalid octect messages that fails during read",
			msgs: []wrp.Union{
				&wrp.Message{
					Source:      "source",
					Destination: "destination",
				},
				&wrp.Message{
					Source:      "source",
					Destination: "destination",
				},
			},
			opts: []Option{
				AsOctetStream(),
			},
			readErr: true,
		},
		{
			name: "invalid msgpackl that fails during read across multiple messages",
			msgs: []wrp.Union{
				&wrp.Message{
					Source:      "source",
					Destination: "destination",
				},
				&wrp.Message{
					Source:      "source",
					Destination: "destination",
				},
			},
			opts: []Option{
				AsMsgpackL(),
				WithMaxItemsPerChunk(1),
			},
			readErr: true,
		},
		{
			name: "invalid jsonl that fails during read with multiple messages",
			msgs: []wrp.Union{
				&wrp.Message{
					Source:      "source",
					Destination: "destination",
				},
				&wrp.Message{
					Source:      "source",
					Destination: "destination",
				},
			},
			opts: []Option{
				AsJSONL(),
			},
			readErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoder, err := NewEncoder(test.opts...)
			require.NoError(t, err)
			require.NotNil(t, encoder)
			headers, body, err := encoder.ToParts(test.msgs...)
			if test.err {
				require.Error(t, err)
				assert.Nil(t, headers)
				assert.Nil(t, body)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, headers)
			assert.NotNil(t, body)

			// Read the body to ensure it is not empty
			n, err := io.Copy(io.Discard, body)
			if test.readErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Greater(t, n, int64(0), "body should not be empty")
		})
	}
}

func TestEncodeRequests(t *testing.T) {
	jsonMsg := testWRPMessages[0]
	jsonMsg.ContentType = "application/json"

	partTypes := func(t *testing.T, req *http.Request) (types, encodings []string) {
		require.True(t, strings.HasPrefix(
			strings.TrimSpace(req.Header.Get("Content-Type")),
			"multipart/mixed;"))

		mp, err := req.MultipartReader()
		require.NoError(t, err)

		for {
			part, err := mp.NextPart()
			if err == io.EOF {
				return types, encodings
			}
			require.NoError(t, err)
			types = append(types, part.Header.Get("Content-Type"))
			encodings = append(encodings, part.Header.Get("Content-Encoding"))
		}
	}

	tests := []struct {
		name  string
		opts  []Option
		msgs  []wrp.Message
		check func(*testing.T, *http.Request)
		err   bool
	}{
		{
			name: "header form sends the payload type, like wrp-go/v3",
			opts: []Option{
				EncodeValidators(wrp.NoStandardValidation()),
				AsOctetStream(),
			},
			msgs: []wrp.Message{jsonMsg},
			check: func(t *testing.T, req *http.Request) {
				assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
				assert.Equal(t, "application/json", req.Header.Get("X-Xmidt-Content-Type"))
			},
		},
		{
			name: "header form without a payload type is octet-stream",
			opts: []Option{
				EncodeValidators(wrp.NoStandardValidation()),
				AsMediaType(MEDIA_TYPE_OCTET_STREAM_XMIDT_STYLE),
			},
			msgs: []wrp.Message{testWRPMessages[0]},
			check: func(t *testing.T, req *http.Request) {
				assert.Equal(t, MEDIA_TYPE_OCTET_STREAM, req.Header.Get("Content-Type"))
				assert.Empty(t, req.Header.Get("X-Xmidt-Content-Type"))
			},
		},
		{
			name: "header form sends each part's payload type, multiple messages",
			opts: []Option{
				EncodeValidators(wrp.NoStandardValidation()),
				AsMediaType(MEDIA_TYPE_OCTET_STREAM_WEBPA_STYLE),
			},
			msgs: []wrp.Message{jsonMsg, testWRPMessages[0]},
			check: func(t *testing.T, req *http.Request) {
				types, encodings := partTypes(t, req)
				assert.Equal(t, []string{"application/json", MEDIA_TYPE_OCTET_STREAM}, types)
				assert.Equal(t, []string{"", ""}, encodings)
			},
		},
		{
			name: "header form sends each part's payload type, multiple messages, gzip",
			opts: []Option{
				EncodeValidators(wrp.NoStandardValidation()),
				AsMediaType(MEDIA_TYPE_OCTET_STREAM_XMIDT_STYLE),
				EncodeGzip(),
			},
			msgs: []wrp.Message{testWRPMessages[0], testWRPMessages[0]},
			check: func(t *testing.T, req *http.Request) {
				types, encodings := partTypes(t, req)
				assert.Equal(t, []string{MEDIA_TYPE_OCTET_STREAM, MEDIA_TYPE_OCTET_STREAM}, types)
				assert.Equal(t, []string{"gzip", "gzip"}, encodings)
			},
		},
		{
			name: "compatibility mode no longer changes anything",
			opts: []Option{
				CompatibilityMode(),
				EncodeValidators(wrp.NoStandardValidation()),
				AsOctetStream(),
			},
			msgs: []wrp.Message{jsonMsg},
			check: func(t *testing.T, req *http.Request) {
				assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoder, err := NewEncoder(test.opts...)
			require.NoError(t, err)
			require.NotNil(t, encoder)

			results, err := encoder.NewRequest(http.MethodPost, "http://example.com", toUnion(test.msgs)...)
			if test.err {
				require.Error(t, err)
				assert.Nil(t, results)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, results)

			if test.check != nil {
				test.check(t, results)
			}
		})
	}
}
