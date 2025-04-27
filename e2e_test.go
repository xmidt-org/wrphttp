// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrphttp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

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

	compat := []testOption{
		{CompatibilityMode(), "CompatibilityMode"},
		{CompatibilityMode(false), "NoCompatibilityMode"},
	}
	for _, tt := range tests {
		for _, typ := range typs {
			for _, encoding := range encodings {
				for _, comp := range compat {
					testName := tt.name + " " + comp.name + "." + typ.name + "." + encoding.name
					t.Run(testName+" Request", func(t *testing.T) {
						t.Parallel()
						opts := append(tt.opts, typ.opt, comp.opt, encoding.opt, EncodeValidators(wrp.NoStandardValidation()))
						// Create an encoder
						encoder, err := NewEncoder(opts...)
						require.NoError(t, err)

						// Encode the messages
						req, err := encoder.NewRequest(http.MethodPost, "http://example.com", toUnion(tt.messages)...)
						require.NoError(t, err)

						ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
						defer cancel()

						req, err = materializeRequest(ctx, req)
						require.NoError(t, err)
						require.NotNil(t, req)

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
}

// Helper function to convert []wrp.Message to []wrp.Union
func toUnion(messages []wrp.Message) []wrp.Union {
	unions := make([]wrp.Union, len(messages))
	for i, msg := range messages {
		unions[i] = &msg
	}
	return unions
}

// This separates the encoding of the request from handling the request.  This
// helps determine if encoding or decoding is the issue.  Otherwise it is safe
// to use the encoder directly on the request & this is a no-op.
func materializeRequest(ctx context.Context, req *http.Request) (*http.Request, error) {
	if req.Body == nil {
		return req, nil
	}

	// Use a cancellable reader
	bodyReader := req.Body
	defer bodyReader.Close()

	pipeReader, pipeWriter := io.Pipe()

	// Copy body into a buffer through a pipe, watching for context cancellation
	go func() {
		defer pipeWriter.Close()

		_, err := io.Copy(pipeWriter, bodyReader)
		if err != nil {
			pipeWriter.CloseWithError(err)
		}
	}()

	var buf bytes.Buffer
	copyDone := make(chan error, 1)

	go func() {
		_, err := io.Copy(&buf, pipeReader)
		copyDone <- err
	}()

	select {
	case <-ctx.Done():
		pipeReader.CloseWithError(ctx.Err())
		return nil, errors.New("materialize canceled")
	case err := <-copyDone:
		if err != nil {
			return nil, err
		}
	}

	// Replace the request body
	req.Body = io.NopCloser(bytes.NewReader(buf.Bytes()))
	req.ContentLength = int64(buf.Len())

	// Clear Transfer-Encoding if present
	if req.Header.Get("Transfer-Encoding") == "chunked" {
		req.Header.Del("Transfer-Encoding")
	}

	return req, nil
}
