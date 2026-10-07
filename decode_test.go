// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrphttp

import (
	"bytes"
	"io"
	"maps"
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
				"Content-Type": []string{"application/json"},
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
				"Content-Type":       []string{"application/octet-stream"},
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
				"Content-Type: application/json\n" +
				"\n" +
				"{\"msg_type\":3,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary\n" +
				"Content-Type: application/json\n" +
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
			name:   "no content type, json body",
			header: http.Header{},
			body:   `{"msg_type":3,"source":"source"}`,
			noVal:  true,
			expected: []wrp.Union{
				&wrp.Message{
					Type:   3,
					Source: "source",
				},
			},
		},
		{
			name: "invalid content type",
			header: http.Header{
				"Content-Type": []string{"multipart/invalid; boundary=boundary"},
			},
			body: "--boundary\n" +
				"Content-Type: application/json\n" +
				"\n" +
				"{\"msg_type\":3,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary\n" +
				"Content-Type: application/json\n" +
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
				"Content-Type: application/json\n" +
				"\n" +
				"{\"msg_type\":3,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary\n" +
				"Content-Type: application/json\n" +
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
				"Content-Type: application/json\n" +
				"Content-Encoding: unknown\n" +
				"\n" +
				"{\"msg_type\":3,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary\n" +
				"Content-Type: application/json\n" +
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
				"Content-Type: application/unknown\n" +
				"\n" +
				"{\"msg_type\":3,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary\n" +
				"Content-Type: application/json\n" +
				"\n" +
				"{\"msg_type\":4,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary--\n",
			noVal: true,
			err:   true,
		},
		{
			name: "multipart part without Content-Type is sniffed",
			header: http.Header{
				"Content-Type": []string{"multipart/mixed; boundary=boundary"},
			},
			body: "--boundary\n" +
				"\n" +
				"{\"msg_type\":3,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary\n" +
				"Content-Type: application/json\n" +
				"\n" +
				"{\"msg_type\":4,\"source\":\"source\"}\n" +
				"\n" +
				"--boundary--\n",
			noVal: true,
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
		},
		{
			name: "json body is invalid",
			header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			body:  "invalid body",
			noVal: true,
			err:   true,
		},
		{
			name: "jsonl body is invalid",
			header: http.Header{
				"Content-Type": []string{"application/jsonl"},
			},
			body:  "invalid body\n",
			noVal: true,
			err:   true,
		},
		{
			name: "msgpackl body is invalid",
			header: http.Header{
				"Content-Type": []string{"application/msgpackl"},
			},
			body:  "invalid body\n",
			noVal: true,
			err:   true,
		},
		{
			name: "msgpackl length is bogus",
			header: http.Header{
				"Content-Type": []string{"application/msgpackl"},
			},
			body:  string([]byte{0x94, 0xFF}),
			noVal: true,
			err:   true,
		},
		{
			name: "invalid octect",
			header: http.Header{
				"Content-Type":       []string{"application/octet-stream"},
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

// TestDecodeV3Forms covers the forms produced by wrp-go/v3/wrphttp.
func TestDecodeV3Forms(t *testing.T) {
	msg := wrp.Message{
		Type:            wrp.SimpleEventMessageType,
		Source:          "dns:source.example.com",
		Destination:     "mac:112233445566",
		TransactionUUID: "uuid",
	}
	var msgpack, json bytes.Buffer
	require.NoError(t, wrp.Msgpack.Encoder(&msgpack).Encode(&msg))
	require.NoError(t, wrp.JSON.Encoder(&json).Encode(&msg))

	headerForm := func(extra http.Header) http.Header {
		h := http.Header{
			"X-Xmidt-Message-Type": []string{"SimpleEvent"},
			"X-Xmidt-Source":       []string{"dns:source.example.com"},
			"X-Webpa-Device-Name":  []string{"mac:112233445566"},
		}
		maps.Copy(h, extra)
		return h
	}
	withHeaders := func(m *wrp.Message, headers ...string) *wrp.Message {
		m.Headers = append([]string{}, headers...)
		return m
	}
	headerMsg := func(ct string, payload []byte, md map[string]string) *wrp.Message {
		return &wrp.Message{
			Type:        wrp.SimpleEventMessageType,
			Source:      "dns:source.example.com",
			Destination: "mac:112233445566",
			ContentType: ct,
			Payload:     payload,
			Metadata:    md,
		}
	}

	tests := []struct {
		name     string
		header   http.Header
		body     string
		expected wrp.Union
	}{
		{
			name:     "no Content-Type, msgpack body (v3 handler)",
			header:   http.Header{},
			body:     msgpack.String(),
			expected: &msg,
		}, {
			name:     "no Content-Type, json body (v3 DecodeRequest)",
			header:   http.Header{},
			body:     json.String(),
			expected: &msg,
		}, {
			name:     "no Content-Type, json body with leading whitespace",
			header:   http.Header{},
			body:     " \r\n\t" + json.String(),
			expected: &msg,
		}, {
			name:     "deprecated application/wrp is msgpack",
			header:   http.Header{"Content-Type": []string{"application/wrp"}},
			body:     msgpack.String(),
			expected: &msg,
		}, {
			name:     "header form with a json payload",
			header:   headerForm(http.Header{"Content-Type": {"application/json"}}),
			body:     `{"hello":"world"}`,
			expected: headerMsg("application/json", []byte(`{"hello":"world"}`), nil),
		}, {
			name:     "header form with a plain octet-stream payload",
			header:   headerForm(http.Header{"Content-Type": {"application/octet-stream"}}),
			body:     "bytes",
			expected: headerMsg("", []byte("bytes"), nil),
		}, {
			name:     "header form with a styled octet-stream envelope",
			header:   headerForm(http.Header{"Content-Type": {"application/octet-stream; style=x-webpa"}}),
			body:     "bytes",
			expected: headerMsg("", []byte("bytes"), nil),
		}, {
			name: "header form content type header wins",
			header: headerForm(http.Header{
				"Content-Type":         {"application/octet-stream; style=x-webpa"},
				"X-Xmidt-Content-Type": {"text/plain"},
			}),
			body:     "bytes",
			expected: headerMsg("text/plain", []byte("bytes"), nil),
		}, {
			name:     "header form without Content-Type or payload",
			header:   headerForm(nil),
			expected: headerMsg("", []byte{}, nil),
		}, {
			name: "header form with legacy X-Midt-Msg-Type",
			header: http.Header{
				"Content-Type":        []string{"application/octet-stream"},
				"X-Midt-Msg-Type":     []string{"SimpleEvent"},
				"X-Midt-Source":       []string{"dns:source.example.com"},
				"X-Webpa-Device-Name": []string{"mac:112233445566"},
			},
			body:     "bytes",
			expected: headerMsg("", []byte("bytes"), nil),
		}, {
			name: "metadata in v3 key=value form",
			header: headerForm(http.Header{
				"X-Xmidt-Metadata": {"/boot-time=123", "url=http://example.com/a=b"},
			}),
			expected: headerMsg("", []byte{}, map[string]string{
				"/boot-time": "123",
				"url":        "http://example.com/a=b",
			}),
		}, {
			name:     "metadata as a comma separated list",
			header:   headerForm(http.Header{"X-Xmidt-Metadata": {"a=1, b=2,c"}}),
			expected: headerMsg("", []byte{}, map[string]string{"a": "1", "b": "2", "c": ""}),
		}, {
			name: "headers, one entry per line",
			header: headerForm(http.Header{
				"X-Xmidt-Headers": {"traceparent: a", "tracestate: b=1,c=2"},
			}),
			expected: withHeaders(headerMsg("", []byte{}, nil), "traceparent: a", "tracestate: b=1,c=2"),
		}, {
			name: "headers folded onto one line by an intermediary",
			header: headerForm(http.Header{
				"X-Xmidt-Headers": {"traceparent: a, tracestate: b=1,c=2", "X-Foo: bar"},
			}),
			expected: withHeaders(headerMsg("", []byte{}, nil), "traceparent: a", "tracestate: b=1,c=2", "X-Foo: bar"),
		}, {
			name:     "headers folded, legacy X-Midt name",
			header:   headerForm(http.Header{"X-Midt-Headers": {"traceparent: a, tracestate: b=1,c=2"}}),
			expected: withHeaders(headerMsg("", []byte{}, nil), "traceparent: a", "tracestate: b=1,c=2"),
		}, {
			name:     "headers folded, Xmidt name",
			header:   headerForm(http.Header{"Xmidt-Headers": {"traceparent: a, tracestate: b=1,c=2"}}),
			expected: withHeaders(headerMsg("", []byte{}, nil), "traceparent: a", "tracestate: b=1,c=2"),
		}, {
			name:     "headers with an empty value",
			header:   headerForm(http.Header{"X-Xmidt-Headers": {""}}),
			expected: withHeaders(headerMsg("", []byte{}, nil)),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := http.Request{
				Header: test.header,
				Body:   io.NopCloser(strings.NewReader(test.body)),
			}
			got, err := DecodeRequest(&req)
			require.NoError(t, err)
			assert.Equal(t, []wrp.Union{test.expected}, got)

			resp := http.Response{
				Header: test.header,
				Body:   io.NopCloser(strings.NewReader(test.body)),
			}
			got, err = DecodeResponse(&resp)
			require.NoError(t, err)
			assert.Equal(t, []wrp.Union{test.expected}, got)
		})
	}
}

func TestSniffFormat(t *testing.T) {
	tests := []struct {
		name string
		body string
		want mediaType
	}{
		{name: "object", body: `{"msg_type":4}`, want: mtJSON},
		{name: "array", body: `[1]`, want: mtJSON},
		{name: "space", body: ` {}`, want: mtJSON},
		{name: "tab", body: "\t{}", want: mtJSON},
		{name: "newline", body: "\n{}", want: mtJSON},
		{name: "carriage return", body: "\r{}", want: mtJSON},
		{name: "fixmap", body: "\x81\xa8msg_type\x04", want: mtMsgpack},
		{name: "fixmap, largest", body: "\x8f", want: mtMsgpack},
		{name: "map16", body: "\xde\x00\x01", want: mtMsgpack},
		{name: "map32", body: "\xdf\x00\x00\x00\x01", want: mtMsgpack},
		{name: "fixarray", body: "\x91\x04", want: mtMsgpack},
		{name: "empty", body: "", want: mtMsgpack},
		{name: "other text", body: "hello", want: mtMsgpack},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			closed := false
			body := struct {
				io.Reader
				io.Closer
			}{strings.NewReader(test.body), closerFunc(func() error { closed = true; return nil })}

			rc, got := sniffFormat(body)
			assert.Equal(t, test.want, got)

			// Peeking consumed nothing and closing reaches the original body.
			rest, err := io.ReadAll(rc)
			require.NoError(t, err)
			assert.Equal(t, test.body, string(rest))
			require.NoError(t, rc.Close())
			assert.True(t, closed)
		})
	}

	t.Run("nil body", func(t *testing.T) {
		rc, got := sniffFormat(nil)
		assert.Nil(t, rc)
		assert.Equal(t, mtMsgpack, got)
	})
}

type closerFunc func() error

func (f closerFunc) Close() error { return f() }

// A JSON body with no Content-Type decodes, and the error for a body that is
// neither names the format that was tried.
func TestDecodeNoContentTypeErrors(t *testing.T) {
	for _, body := range []string{"", "hello", "{not json"} {
		t.Run(body, func(t *testing.T) {
			_, err := DecodeRequest(&http.Request{
				Header: http.Header{},
				Body:   io.NopCloser(strings.NewReader(body)),
			})
			require.Error(t, err)
		})
	}
}

// decodeNoValidation decodes body with the given Content-Type, or none.
func decodeNoValidation(ct string, body []byte) ([]wrp.Union, error) {
	h := http.Header{}
	if ct != "" {
		h.Set("Content-Type", ct)
	}
	return DecodeRequest(&http.Request{
		Header: h,
		Body:   io.NopCloser(bytes.NewReader(body)),
	}, wrp.NoStandardValidation())
}

// FuzzDecodeNoContentType checks that sniffing never loses a message: a body
// without a Content-Type decodes to exactly what it decodes to with the right
// Content-Type, and fails only when neither format reads it.  It also checks
// that no body is both valid JSON and valid msgpack.
func FuzzDecodeNoContentType(f *testing.F) {
	msg := wrp.Message{Type: wrp.SimpleEventMessageType, Source: "dns:a.example.com", Destination: "mac:112233445566", Payload: []byte("hi")}
	var mp, js bytes.Buffer
	require.NoError(f, wrp.Msgpack.Encoder(&mp).Encode(&msg))
	require.NoError(f, wrp.JSON.Encoder(&js).Encode(&msg))

	for _, seed := range [][]byte{
		nil, []byte("hello"), []byte("{"), []byte("[1]"), []byte("\xde"), []byte("\x80"),
		mp.Bytes(), js.Bytes(), append([]byte(" \r\n\t"), js.Bytes()...), []byte(`{"msg_type":4}`),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, body []byte) {
		asJSON, jsonErr := decodeNoValidation(MEDIA_TYPE_JSON, body)
		asMsgpack, msgpackErr := decodeNoValidation(MEDIA_TYPE_MSGPACK, body)
		require.False(t, jsonErr == nil && msgpackErr == nil, "a body can't be both JSON and msgpack")

		got, err := decodeNoValidation("", body)
		switch {
		case jsonErr == nil:
			require.NoError(t, err, "json body not read without a Content-Type")
			assert.Equal(t, asJSON, got)
		case msgpackErr == nil:
			require.NoError(t, err, "msgpack body not read without a Content-Type")
			assert.Equal(t, asMsgpack, got)
		default:
			require.Error(t, err)
		}
	})
}

// FuzzDecodeNoContentTypeRoundTrip encodes a message as JSON and as msgpack,
// sends each without a Content-Type, and expects the message back.
func FuzzDecodeNoContentTypeRoundTrip(f *testing.F) {
	f.Add(uint8(4), "dns:a", "mac:b", "tid", []byte("payload"), "traceparent: a", "p1", uint8(0))
	f.Add(uint8(3), "", "", "", []byte{}, "", "", uint8(1))
	f.Add(uint8(11), "mac:112233445566", "event:device-status", "", []byte{0x80, 0xde, '{'}, "tracestate: a=1,b=2", "sky", uint8(4))

	leading := []string{"", " ", "\t", "\n", "\r\n", " \t\r\n "}

	f.Fuzz(func(t *testing.T, typ uint8, source, dest, tid string, payload []byte, header, partner string, ws uint8) {
		// JSON replaces invalid UTF-8 with U+FFFD, which is a property of
		// JSON and not of what is under test here.
		for _, s := range []*string{&source, &dest, &tid, &header, &partner} {
			*s = strings.ToValidUTF8(*s, "")
		}
		msg := wrp.Message{
			Type:            wrp.MessageType(typ),
			Source:          source,
			Destination:     dest,
			TransactionUUID: tid,
		}
		if len(payload) > 0 {
			msg.Payload = payload
		}
		if header != "" {
			msg.Headers = []string{header}
		}
		if partner != "" {
			msg.PartnerIDs = []string{partner}
		}

		for _, format := range []wrp.Format{wrp.JSON, wrp.Msgpack} {
			var body bytes.Buffer
			if format == wrp.JSON {
				body.WriteString(leading[int(ws)%len(leading)])
			}
			require.NoError(t, format.Encoder(&body).Encode(&msg, wrp.NoStandardValidation()))

			got, err := decodeNoValidation("", body.Bytes())
			require.NoError(t, err, "%s body: %q", format, body.Bytes())
			require.Len(t, got, 1)
			var out wrp.Message
			require.NoError(t, got[0].To(&out, wrp.NoStandardValidation()))
			assert.Equal(t, msg, out, "%s", format)
		}
	})
}
