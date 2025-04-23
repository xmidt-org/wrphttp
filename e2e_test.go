// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrphttp

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/wrp-go/v5"
)

var testWRPMessages = []wrp.Message{
	{
		Type:            wrp.SimpleRequestResponseMessageType,
		Source:          "source1",
		Destination:     "destination1",
		TransactionUUID: "uuid1",
		Status: func() *int64 {
			i := int64(200)
			return &i
		}(),
		PartnerIDs: []string{"partner1", "partner2"},
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		Headers: []string{
			"header1",
			"header2",
		},
		Payload: []byte("payload1"),
	},
	{
		Type:            wrp.SimpleRequestResponseMessageType,
		Source:          "source2",
		Destination:     "destination2",
		TransactionUUID: "uuid2",
		Payload:         []byte("payload2"),
	},
	{
		Type:            wrp.SimpleEventMessageType,
		Source:          "source3",
		Destination:     "destination3",
		TransactionUUID: "uuid3",
		Payload:         []byte("payload3"),
	},
}

type testOption struct {
	opt  Option
	name string
}

func TestEncodeDecodeWRPMessages(t *testing.T) {
	tests := []struct {
		name     string
		messages []wrp.Message
		opts     []Option
	}{
		{
			name: "single message",
			messages: []wrp.Message{
				testWRPMessages[0],
			},
		}, {
			name: "multiple messages",
			messages: []wrp.Message{
				testWRPMessages[0],
				testWRPMessages[1],
			},
		}, {
			name: "multiple messages with limit",
			messages: []wrp.Message{
				testWRPMessages[0],
				testWRPMessages[1],
				testWRPMessages[2],
			},
			opts: []Option{
				WithMaxItemsPerChunk(2),
			},
		},
	}

	typs := []testOption{
		{AsJSON(), "AsJSON"},
		{AsJSONL(), "AsJSONL"},
		{AsMsgpack(), "AsMsgpack"},
		{AsMsgpackL(), "AsMsgpackL"},
		{AsOctetStream(), "AsOctetStream"},
		{AsOctetStream("X-Xmidt"), "AsOctetStream(X-Xmidt)"},
		{AsOctetStream("X-Midt"), "AsOctetStream(X-Midt)"},
		{AsOctetStream("Xmidt"), "AsOctetStream(Xmidt)"},
		{AsOctetStream("X-Webpa"), "AsOctetStream(X-Webpa)"},
	}

	encodings := []testOption{
		{EncodeNoCompression(), "EncodeNoCompression"},
		{EncodeGzip(), "EncodeGzip"},
		{EncodeDeflate(), "EncodeDeflate"},
		{EncodeZlib(), "EncodeZlib"},
	}
	for _, tt := range tests {
		for _, typ := range typs {
			for _, encoding := range encodings {
				testName := tt.name + " " + typ.name + "." + encoding.name
				t.Run(testName+" Request", func(t *testing.T) {
					opts := append(tt.opts, typ.opt, encoding.opt, EncodeValidators(wrp.NoStandardValidation()))
					// Create an encoder
					encoder, err := NewEncoder(opts...)
					require.NoError(t, err)

					// Encode the messages
					req, err := encoder.NewRequest(http.MethodPost, "http://example.com", toUnion(tt.messages)...)
					require.NoError(t, err)

					got, err := DecodeRequest(req, wrp.NoStandardValidation())
					require.NoError(t, err)
					require.Len(t, got, len(tt.messages))
					for i, original := range tt.messages {
						decoded := got[i].(*wrp.Message)
						assert.Equal(t, original, *decoded)
					}
				})
				t.Run(testName+" Response", func(t *testing.T) {
					opts := append(tt.opts, typ.opt, encoding.opt, EncodeValidators(wrp.NoStandardValidation()))
					// Create an encoder
					encoder, err := NewEncoder(opts...)
					require.NoError(t, err)

					// Encode the messages
					headers, body, err := encoder.ToParts(toUnion(tt.messages)...)
					require.NoError(t, err)
					require.NotNil(t, body)
					require.NotNil(t, headers)

					// Simulate sending the request
					resp := &http.Response{
						Header: headers,
						Body:   io.NopCloser(body),
					}

					// Decode the messages
					decodedMessages, err := DecodeResponse(resp, wrp.NoStandardValidation())
					require.NoError(t, err)

					// Compare the original and decoded messages
					require.Len(t, decodedMessages, len(tt.messages))
					for i, original := range tt.messages {
						decoded := decodedMessages[i].(*wrp.Message)
						assert.Equal(t, original, *decoded)
					}
				})
			}
		}
	}
}

// Helper function to convert []wrp.Message to []wrp.Union
func toUnion(messages []wrp.Message) []wrp.Union {
	unions := make([]wrp.Union, len(messages))
	for i, msg := range messages {
		unions[i] = &msg
	}
	return unions
}
