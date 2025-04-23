// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrphttp

import (
	"context"
	"io"
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
