// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrphttp

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/wrp-go/v5"
)

func TestFrom(t *testing.T) {

	tests := []struct {
		name     string
		ptrNil   bool
		header   http.Header
		body     string
		expected []wrp.Union
		noVal    bool
		err      bool
	}{
		// Valid/happy cases
		{
			name: "valid json",
			header: http.Header{
				"Content-Type": []string{"application/wrp+json"},
			},
			body:  `{"msg_type":3,"source":"source"}`,
			noVal: true,
			expected: []wrp.Union{
				&wrp.Message{
					Type:   3,
					Source: "source",
				},
			},
			err: false,
		},
		{
			name: "valid octect",
			header: http.Header{
				"Content-Type":       []string{"application/wrp+octet-stream"},
				"Xmidt-Message-Type": []string{"Unknown"},
			},
			expected: []wrp.Union{
				&wrp.Message{
					Type:    wrp.UnknownMessageType,
					Payload: []byte{},
				},
			},
			err: false,
		},
		{
			name: "valid json",
			header: http.Header{
				"Content-Type": []string{"multipart/mixed; boundary=boundary"},
			},
			body: "--boundary\n" +
				"Content-Type: application/wrp+json\n" +
				"\n" +
				"{\"msg_type\":3,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary\n" +
				"Content-Type: application/wrp+json\n" +
				"\n" +
				"{\"msg_type\":4,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary--\n",
			expected: []wrp.Union{
				&wrp.Message{
					Type:   wrp.SimpleRequestResponseMessageType,
					Source: "source",
				},
				&wrp.Message{
					Type:   wrp.SimpleEventMessageType,
					Source: "source",
				},
			},
			noVal: true,
			err:   false,
		},

		// Invalid cases

		{
			name: "no headers",
			body: `{"msg_type":3,"source":"source"}`,
			err:  true,
		}, {
			name:   "no content type",
			header: http.Header{},
			body:   `{"msg_type":3,"source":"source"}`,
			noVal:  true,
			err:    true,
		},
		{
			name: "invalid content type",
			header: http.Header{
				"Content-Type": []string{"multipart/invalid; boundary=boundary"},
			},
			body: "--boundary\n" +
				"Content-Type: application/wrp+json\n" +
				"\n" +
				"{\"msg_type\":3,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary\n" +
				"Content-Type: application/wrp+json\n" +
				"\n" +
				"{\"msg_type\":4,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary--\n",
			noVal: true,
			err:   true,
		},
		{
			name: "invalid content type - no boundary",
			header: http.Header{
				"Content-Type": []string{"multipart/mixed; dogs=cats"},
			},
			body: "--boundary\n" +
				"Content-Type: application/wrp+json\n" +
				"\n" +
				"{\"msg_type\":3,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary\n" +
				"Content-Type: application/wrp+json\n" +
				"\n" +
				"{\"msg_type\":4,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary--\n",
			noVal: true,
			err:   true,
		},
		{
			name:   "nil",
			ptrNil: true,
			err:    true,
		}, {
			name: "invalid payload",
			header: http.Header{
				"Content-Type": []string{"multipart/mixed; boundary=boundary"},
			},
			body:  "--boundary",
			noVal: true,
			err:   true,
		},
		{
			name: "unknown encoding",
			header: http.Header{
				"Content-Type": []string{"multipart/mixed; boundary=boundary"},
			},
			body: "--boundary\n" +
				"Content-Type: application/wrp+json\n" +
				"Content-Encoding: unknown\n" +
				"\n" +
				"{\"msg_type\":3,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary\n" +
				"Content-Type: application/wrp+json\n" +
				"\n" +
				"{\"msg_type\":4,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary--\n",
			noVal: true,
			err:   true,
		},
		{
			name: "unknown multipart Content-Type",
			header: http.Header{
				"Content-Type": []string{"multipart/mixed; boundary=boundary"},
			},
			body: "--boundary\n" +
				"Content-Type: application/wrp+unknown\n" +
				"\n" +
				"{\"msg_type\":3,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary\n" +
				"Content-Type: application/wrp+json\n" +
				"\n" +
				"{\"msg_type\":4,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary--\n",
			noVal: true,
			err:   true,
		},
		{
			name: "missing multipart Content-Type",
			header: http.Header{
				"Content-Type": []string{"multipart/mixed; boundary=boundary"},
			},
			body: "--boundary\n" +
				"\n" +
				"{\"msg_type\":3,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary\n" +
				"Content-Type: application/wrp+json\n" +
				"\n" +
				"{\"msg_type\":4,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary--\n",
			noVal: true,
			err:   true,
		},
		{
			name: "json body is invalid",
			header: http.Header{
				"Content-Type": []string{"application/wrp+json"},
			},
			body:  "invalid body",
			noVal: true,
			err:   true,
		},
		{
			name: "jsonl body is invalid",
			header: http.Header{
				"Content-Type": []string{"application/wrp+jsonl"},
			},
			body:  "invalid body\n",
			noVal: true,
			err:   true,
		},
		{
			name: "msgpackl body is invalid",
			header: http.Header{
				"Content-Type": []string{"application/wrp+msgpackl"},
			},
			body:  "invalid body\n",
			noVal: true,
			err:   true,
		},
		{
			name: "msgpackl length is bogus",
			header: http.Header{
				"Content-Type": []string{"application/wrp+msgpackl"},
			},
			body:  string([]byte{0x94, 0xFF}),
			noVal: true,
			err:   true,
		},
		{
			name: "invalid octect",
			header: http.Header{
				"Content-Type":       []string{"application/wrp+octet-stream"},
				"Xmidt-Message-Type": []string{"Unknown"},
				"Xmidt-Status":       []string{"1"},
				"Xmidt-URL":          []string{"url"},
			},
			err: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.ptrNil {
				result, err := DecodeRequest(nil)
				require.Error(t, err)
				assert.Nil(t, result)

				got, err := DecodeResponse(nil)
				require.Error(t, err)
				assert.Nil(t, got)
				return
			}

			var validators []wrp.Processor
			if test.noVal {
				validators = append(validators, wrp.NoStandardValidation())
			}

			t.Run(test.name+" Request", func(t *testing.T) {
				req := http.Request{
					Header: test.header,
					Body:   io.NopCloser(strings.NewReader(test.body)),
				}
				result, err := DecodeRequest(&req, validators...)
				if test.err {
					require.Error(t, err)
					assert.Nil(t, result)
				} else {
					require.NoError(t, err)
					assert.Equal(t, test.expected, result)
				}
			})

			t.Run(test.name+" Response", func(t *testing.T) {
				resp := http.Response{
					Header: test.header,
					Body:   io.NopCloser(strings.NewReader(test.body)),
				}
				result, err := DecodeResponse(&resp, validators...)
				if test.err {
					require.Error(t, err)
					assert.Nil(t, result)
				} else {
					require.NoError(t, err)
					assert.Equal(t, test.expected, result)
				}
			})
		})
	}
}
