// SPDX-FileCopyrightText: 2026 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package compat

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v3 "github.com/xmidt-org/wrp-go/v3"
	v3http "github.com/xmidt-org/wrp-go/v3/wrphttp"
	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/wrphttp"
)

func i64(i int64) *int64 { return &i }

// testMessage returns a message with every field a Retrieve may carry.
func testMessage() *wrp.Message {
	return &wrp.Message{
		Type:                    wrp.RetrieveMessageType,
		Source:                  "dns:talaria.example.com",
		Destination:             "mac:112233445566/config",
		TransactionUUID:         "tx-123",
		ContentType:             "application/json",
		Accept:                  "application/json",
		Status:                  i64(200),
		RequestDeliveryResponse: i64(1),
		Headers:                 []string{"X-Foo: bar"},
		Metadata:                map[string]string{"/boot-time": "123", "url": "http://example.com"},
		Path:                    "/a/b",
		Payload:                 []byte(`{"hello":"world"}`),
		PartnerIDs:              []string{"comcast", "sky"},
		SessionID:               "sess-1",
		QualityOfService:        50,
	}
}

func toV3(m *wrp.Message) *v3.Message {
	return &v3.Message{
		Type:                    v3.MessageType(m.Type),
		Source:                  m.Source,
		Destination:             m.Destination,
		TransactionUUID:         m.TransactionUUID,
		ContentType:             m.ContentType,
		Accept:                  m.Accept,
		Status:                  m.Status,
		RequestDeliveryResponse: m.RequestDeliveryResponse,
		Headers:                 m.Headers,
		Metadata:                m.Metadata,
		Path:                    m.Path,
		Payload:                 m.Payload,
		ServiceName:             m.ServiceName,
		URL:                     m.URL,
		PartnerIDs:              m.PartnerIDs,
		SessionID:               m.SessionID,
		QualityOfService:        v3.QOSValue(m.QualityOfService),
	}
}

func fromV3(m *v3.Message) *wrp.Message {
	return &wrp.Message{
		Type:                    wrp.MessageType(m.Type),
		Source:                  m.Source,
		Destination:             m.Destination,
		TransactionUUID:         m.TransactionUUID,
		ContentType:             m.ContentType,
		Accept:                  m.Accept,
		Status:                  m.Status,
		RequestDeliveryResponse: m.RequestDeliveryResponse,
		Headers:                 m.Headers,
		Metadata:                m.Metadata,
		Path:                    m.Path,
		Payload:                 m.Payload,
		ServiceName:             m.ServiceName,
		URL:                     m.URL,
		PartnerIDs:              m.PartnerIDs,
		SessionID:               m.SessionID,
		QualityOfService:        wrp.QOSValue(m.QualityOfService),
	}
}

// headerForm returns the fields that survive the header form, which carries
// neither QOS nor an octet-stream content type.
func headerForm(m *wrp.Message) *wrp.Message {
	m.QualityOfService = 0
	if m.ContentType == wrphttp.MEDIA_TYPE_OCTET_STREAM {
		m.ContentType = ""
	}
	return m
}

// equal compares messages, returning an error instead of failing so known gaps
// can be detected.
func equal(want, got *wrp.Message) error {
	if !assert.ObjectsAreEqual(want, got) {
		return fmt.Errorf("messages differ:\n want: %+v\n  got: %+v", *want, *got)
	}
	return nil
}

// check runs a case that returns an error on failure.  A case marked with a
// known gap is skipped while the gap reproduces and fails once it does not.
func check(t *testing.T, gap string, err error) {
	t.Helper()
	switch {
	case gap == "":
		require.NoError(t, err)
	case err == nil:
		t.Errorf("known gap (%s) no longer reproduces, remove the marker", gap)
	default:
		t.Skipf("known gap (%s): %v", gap, err)
	}
}

// v3Request builds requests the way wrp-go/v3 users do.
type v3Request func(m *v3.Message) *http.Request

func v3EncodedBody(f v3.Format, ct string) v3Request {
	return func(m *v3.Message) *http.Request {
		var b []byte
		if err := v3.NewEncoderBytes(&b, f).Encode(m); err != nil {
			panic(err)
		}
		r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
		if ct != "" {
			r.Header.Set("Content-Type", ct)
		}
		return r
	}
}

func v3HeaderForm(legacy bool) v3Request {
	return func(m *v3.Message) *http.Request {
		var body bytes.Buffer
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		v3http.AddMessageHeaders(r.Header, m)
		if _, err := v3http.WritePayload(r.Header, &body, m); err != nil {
			panic(err)
		}
		r.Body = io.NopCloser(&body)
		if legacy {
			// The deprecated names v3 still reads.
			for k, v := range r.Header {
				if !strings.HasPrefix(k, "X-Xmidt-") {
					continue
				}
				r.Header.Del(k)
				k = strings.Replace(k, "X-Xmidt-", "X-Midt-", 1)
				if k == "X-Midt-Message-Type" {
					k = "X-Midt-Msg-Type"
				}
				r.Header[k] = v
			}
		}
		return r
	}
}

func TestV3RequestToDecodeRequest(t *testing.T) {
	tests := []struct {
		name       string
		request    v3Request
		modify     func(*wrp.Message)
		headerForm bool
		gap        string
	}{
		{name: "msgpack", request: v3EncodedBody(v3.Msgpack, wrphttp.MEDIA_TYPE_MSGPACK)},
		{name: "json", request: v3EncodedBody(v3.JSON, wrphttp.MEDIA_TYPE_JSON)},
		{name: "msgpack without Content-Type", request: v3EncodedBody(v3.Msgpack, "")},
		{name: "msgpack as application/wrp", request: v3EncodedBody(v3.Msgpack, "application/wrp")},
		{name: "header form, json payload", request: v3HeaderForm(false), headerForm: true},
		{
			name:       "header form, octet-stream payload",
			request:    v3HeaderForm(false),
			modify:     func(m *wrp.Message) { m.ContentType = wrphttp.MEDIA_TYPE_OCTET_STREAM },
			headerForm: true,
		}, {
			name:       "header form, no payload",
			request:    v3HeaderForm(false),
			modify:     func(m *wrp.Message) { m.ContentType, m.Payload = "", nil },
			headerForm: true,
		}, {
			name:       "header form, legacy header names",
			request:    v3HeaderForm(true),
			headerForm: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sent := testMessage()
			if tc.modify != nil {
				tc.modify(sent)
			}
			want := testMessage()
			if tc.modify != nil {
				tc.modify(want)
			}
			if tc.headerForm {
				want = headerForm(want)
				if want.Payload == nil {
					want.Payload = []byte{}
				}
			}

			got, err := wrphttp.DecodeRequest(tc.request(toV3(sent)))
			if err == nil {
				require.Len(t, got, 1)
				var msg wrp.Message
				require.NoError(t, got[0].To(&msg))
				err = equal(want, &msg)
			}
			check(t, tc.gap, err)
		})
	}
}

// v3 accepts fields on any message type; v5 validation does not.
func TestV3LenientFields(t *testing.T) {
	m := toV3(testMessage())
	m.Type = v3.SimpleRequestResponseMessageType // Path is not allowed

	_, err := wrphttp.DecodeRequest(v3EncodedBody(v3.Msgpack, wrphttp.MEDIA_TYPE_MSGPACK)(m))
	require.Error(t, err)

	got, err := wrphttp.DecodeRequest(v3EncodedBody(v3.Msgpack, wrphttp.MEDIA_TYPE_MSGPACK)(m), wrp.NoStandardValidation())
	require.NoError(t, err)
	require.Len(t, got, 1)
}

// serveV3 runs the request through a v3 handler and returns what it decoded.
func serveV3(dec v3http.Decoder, req *http.Request) (*wrp.Message, error) {
	var got *wrp.Message
	h := v3http.NewHTTPHandler(v3http.HandlerFunc(func(w v3http.ResponseWriter, r *v3http.Request) {
		got = fromV3(&r.Entity.Message)
		w.WriteHeader(http.StatusOK)
	}), v3http.WithDecoder(dec))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got == nil {
		return nil, fmt.Errorf("v3 rejected the request: %d %s", rec.Code, strings.TrimSpace(rec.Body.String()))
	}
	return got, nil
}

func TestEncoderToV3Handler(t *testing.T) {
	headers := v3http.DecodeEntityFromSources(v3.Msgpack, true)

	tests := []struct {
		name       string
		opts       []wrphttp.Option
		decoder    v3http.Decoder
		count      int
		headerForm bool
		gap        string
	}{
		{name: "msgpack", opts: []wrphttp.Option{wrphttp.AsMsgpack()}},
		{name: "json", opts: []wrphttp.Option{wrphttp.AsJSON()}},
		{
			name:       "octet-stream",
			opts:       []wrphttp.Option{wrphttp.AsOctetStream()},
			decoder:    headers,
			headerForm: true,
		},
		// v5-only forms, documented as such.
		{
			name:    "octet-stream, x-xmidt style",
			opts:    []wrphttp.Option{wrphttp.AsOctetStream("x-xmidt")},
			decoder: headers, headerForm: true,
			gap: "v5-only: x-xmidt header style",
		}, {
			name:  "multiple messages",
			opts:  []wrphttp.Option{wrphttp.AsMsgpack()},
			count: 2,
			gap:   "v5-only: multiple messages",
		}, {
			name: "msgpackl",
			opts: []wrphttp.Option{wrphttp.AsMsgpackL()},
			gap:  "v5-only: msgpackl",
		}, {
			name: "gzip",
			opts: []wrphttp.Option{wrphttp.AsMsgpack(), wrphttp.EncodeGzip()},
			gap:  "v5-only: compression",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			enc, err := wrphttp.NewEncoder(tc.opts...)
			require.NoError(t, err)

			msgs := []wrp.Union{testMessage()}
			for i := 1; i < tc.count; i++ {
				msgs = append(msgs, testMessage())
			}
			req, err := enc.NewRequest(http.MethodPost, "http://example.com/", msgs...)
			require.NoError(t, err)

			dec := tc.decoder
			if dec == nil {
				dec = v3http.DefaultDecoder()
			}

			want := testMessage()
			if tc.headerForm {
				want = headerForm(want)
			}

			got, err := serveV3(dec, req)
			if err == nil {
				err = equal(want, got)
			}
			check(t, tc.gap, err)
		})
	}
}

func TestV3ResponseToDecodeResponse(t *testing.T) {
	for _, accept := range []string{"", wrphttp.MEDIA_TYPE_MSGPACK, wrphttp.MEDIA_TYPE_JSON} {
		t.Run("Accept="+accept, func(t *testing.T) {
			req := v3EncodedBody(v3.Msgpack, wrphttp.MEDIA_TYPE_MSGPACK)(toV3(testMessage()))
			if accept != "" {
				req.Header.Set("Accept", accept)
			}

			h := v3http.NewHTTPHandler(v3http.HandlerFunc(func(w v3http.ResponseWriter, r *v3http.Request) {
				_, _ = w.WriteWRP(&v3http.Entity{Message: *toV3(testMessage())})
			}))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			got, err := wrphttp.DecodeResponse(rec.Result())
			require.NoError(t, err)
			require.Len(t, got, 1)

			var msg wrp.Message
			require.NoError(t, got[0].To(&msg))
			require.NoError(t, equal(testMessage(), &msg))
		})
	}
}

// A v3 client picks the response format with v3.FormatFromContentType.
func TestNegotiatedResponseToV3Client(t *testing.T) {
	tests := []struct {
		accept string
		gap    string
	}{
		{accept: ""},
		{accept: wrphttp.MEDIA_TYPE_MSGPACK},
		{accept: wrphttp.MEDIA_TYPE_JSON},
		{accept: "application/wrp"},
	}

	for _, tc := range tests {
		t.Run("Accept="+tc.accept, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.Header.Set("Content-Type", wrphttp.MEDIA_TYPE_MSGPACK)
			if tc.accept != "" {
				req.Header.Set("Accept", tc.accept)
			}

			enc, err := wrphttp.NewEncoder(wrphttp.AsNegotiated(req))
			require.NoError(t, err)

			h, body, err := enc.ToParts(testMessage())
			require.NoError(t, err)
			b, err := io.ReadAll(body)
			require.NoError(t, err)

			err = func() error {
				f, err := v3.FormatFromContentType(h.Get("Content-Type"), v3.Msgpack)
				if err != nil {
					return err
				}
				var got v3.Message
				if err := v3.NewDecoderBytes(b, f).Decode(&got); err != nil {
					return fmt.Errorf("Content-Type %q: %w", h.Get("Content-Type"), err)
				}
				return equal(testMessage(), fromV3(&got))
			}()
			check(t, tc.gap, err)
		})
	}
}

// Accept: */* negotiates msgpackl, which v3 clients cannot read.  v3 clients
// never send */*, but if one does it must fail rather than decode a wrong
// message.  It always fails because each msgpackl item is binary, which v3
// cannot decode as the message type.
func TestMsgpackLFailsInV3Client(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", wrphttp.MEDIA_TYPE_MSGPACK)
	req.Header.Set("Accept", "*/*")

	mt, err := wrphttp.NegotiateMediaType(req)
	require.NoError(t, err)
	require.Equal(t, wrphttp.MEDIA_TYPE_MSGPACKL, mt)

	for _, count := range []int{1, 3, 20} {
		for _, size := range []int{0, 10, 300, 70000} {
			t.Run(fmt.Sprintf("count=%d,size=%d", count, size), func(t *testing.T) {
				enc, err := wrphttp.NewEncoder(wrphttp.AsNegotiated(req))
				require.NoError(t, err)

				msgs := make([]wrp.Union, 0, count)
				for range count {
					m := testMessage()
					m.Payload = bytes.Repeat([]byte("x"), size)
					msgs = append(msgs, m)
				}

				h, body, err := enc.ToParts(msgs...)
				require.NoError(t, err)
				b, err := io.ReadAll(body)
				require.NoError(t, err)

				f, err := v3.FormatFromContentType(h.Get("Content-Type"))
				require.NoError(t, err)
				require.Equal(t, v3.Msgpack, f, "v3 mistakes msgpackl for msgpack")

				var got v3.Message
				assert.Error(t, v3.NewDecoderBytes(b, f).Decode(&got))
			})
		}
	}
}
