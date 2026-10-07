// SPDX-FileCopyrightText: 2026 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrphttp

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/wrp-go/v5"
)

func TestSplitHeaderEntries(t *testing.T) {
	tests := []struct {
		line string
		want []string
	}{
		{line: "", want: nil},
		{line: " , ", want: nil},
		{line: "traceparent: a", want: []string{"traceparent: a"}},
		{line: "name:value", want: []string{"name:value"}},
		{
			// Two entries folded onto one line.
			line: "traceparent: a, tracestate: b=1",
			want: []string{"traceparent: a", "tracestate: b=1"},
		}, {
			// Folded without a space, or with a tab.
			line: "traceparent: a,tracestate: b=1,\tX-Foo: bar",
			want: []string{"traceparent: a", "tracestate: b=1", "X-Foo: bar"},
		}, {
			// A tracestate is itself a comma separated list.
			line: "tracestate: vendor1=a,vendor2=b, vendor3=c:d",
			want: []string{"tracestate: vendor1=a,vendor2=b, vendor3=c:d"},
		}, {
			// Both at once.
			line: "traceparent: a, tracestate: b=1,c=2, X-Foo: bar",
			want: []string{"traceparent: a", "tracestate: b=1,c=2", "X-Foo: bar"},
		}, {
			// Values that contain commas but don't look like a new entry.
			line: "Date: Tue, 06 Oct 2026 12:00:00 GMT",
			want: []string{"Date: Tue, 06 Oct 2026 12:00:00 GMT"},
		}, {
			line: "Accept: text/html, application/xhtml+xml",
			want: []string{"Accept: text/html, application/xhtml+xml"},
		}, {
			line: "Via: 1.1 a:8080, 1.1 b:8080",
			want: []string{"Via: 1.1 a:8080, 1.1 b:8080"},
		}, {
			// Entries that aren't "name: value" are left alone.
			line: "header1, header2",
			want: []string{"header1, header2"},
		}, {
			// Empty entries are dropped.
			line: ", X-Foo: bar, , X-Bar: baz,",
			want: []string{"X-Foo: bar", "X-Bar: baz"},
		}, {
			// The limitation: a value holding ", name:" splits.
			line: "X-List: a:1, b:2",
			want: []string{"X-List: a:1", "b:2"},
		},
	}

	for _, test := range tests {
		t.Run(test.line, func(t *testing.T) {
			assert.Equal(t, test.want, splitHeaderEntries(test.line))
		})
	}
}

// W3C Trace Context values, which are what candlelight carries in Headers.
const (
	traceparent = "traceparent: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	tracestate  = "tracestate: congo=t61rcWkgMzE,rojo=00f067aa0ba902b7, tenant@vendor=val:ue"
)

func TestSplitHeaderEntriesW3C(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
	}{
		{name: "traceparent", line: traceparent, want: []string{traceparent}},
		{name: "tracestate", line: tracestate, want: []string{tracestate}},
		{name: "folded", line: traceparent + ", " + tracestate, want: []string{traceparent, tracestate}},
		{name: "folded, tracestate first", line: tracestate + "," + traceparent, want: []string{tracestate, traceparent}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, splitHeaderEntries(test.line))
		})
	}
}

// Folding entries the way an intermediary does and splitting them again
// gives the entries back, whatever separator the intermediary used.
func TestSplitHeaderEntriesUndoesFolding(t *testing.T) {
	lists := [][]string{
		{traceparent},
		{traceparent, tracestate},
		{tracestate, traceparent, "X-Foo: bar"},
		{"Date: Tue, 06 Oct 2026 12:00:00 GMT", "Accept: text/html, application/xhtml+xml", "Via: 1.1 a:8080, 1.1 b:8080"},
		{"name:value", "other:value"},
	}
	for _, sep := range []string{",", ", ", " , ", ",\t", ",,"} {
		for _, entries := range lists {
			line := strings.Join(entries, sep)
			t.Run(line, func(t *testing.T) {
				assert.Equal(t, entries, splitHeaderEntries(line))
			})
		}
	}
}

func FuzzSplitHeaderEntries(f *testing.F) {
	for _, seed := range []string{
		"", ",", " , ", "a: b", "a: b, c: d", "a: b,c: d,e: f",
		traceparent + ", " + tracestate, "Date: Tue, 06 Oct 2026 12:00:00 GMT",
		"header1, header2", "X-List: a:1, b:2", ", X-Foo: bar, , X-Bar: baz,", "a: b,\nc: d",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, line string) {
		entries := splitHeaderEntries(line)
		for _, e := range entries {
			// Entries are trimmed, non-empty and don't contain a boundary.
			assert.NotEmpty(t, e)
			assert.Equal(t, strings.Trim(e, " \t"), e)
			assert.Equal(t, []string{e}, splitHeaderEntries(e))
		}
		// Folding the result and splitting it again changes nothing.
		assert.Equal(t, entries, splitHeaderEntries(strings.Join(entries, ", ")))
		assert.Equal(t, entries, splitHeaderEntries(strings.Join(entries, ",")))
	})
}

// TestHeadersSurviveProxyFolding encodes a message in header form, folds every
// repeated header line into one comma separated line the way an intermediary
// may (RFC 9110 §5.3), and decodes it again.  Headers, Metadata and PartnerIDs
// all come back as sent.
func TestHeadersSurviveProxyFolding(t *testing.T) {
	msg := wrp.Message{
		Type:            wrp.SimpleEventMessageType,
		Source:          "dns:source.example.com",
		Destination:     "mac:112233445566",
		TransactionUUID: "uuid",
		ContentType:     "application/json",
		Headers:         []string{traceparent, tracestate, "X-Foo: bar"},
		Metadata:        map[string]string{"a": "1", "b": "2", "c": "3"},
		PartnerIDs:      []string{"p1", "p2"},
		Payload:         []byte(`{"hello":"world"}`),
	}

	for _, style := range []string{"X-Webpa", "X-Xmidt", "X-Midt", "Xmidt"} {
		t.Run(style, func(t *testing.T) {
			enc, err := NewEncoder(AsOctetStream(style))
			require.NoError(t, err)
			req, err := enc.NewRequest(http.MethodPost, "http://example.com", &msg)
			require.NoError(t, err)

			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			req.Body = io.NopCloser(bytes.NewReader(body))

			folded := 0
			for k, v := range req.Header {
				if len(v) > 1 {
					req.Header.Set(k, strings.Join(v, ", "))
					folded++
				}
			}
			require.GreaterOrEqual(t, folded, 2, "the Headers and Metadata lines should have been folded")

			got, err := DecodeRequest(req)
			require.NoError(t, err)
			require.Len(t, got, 1)
			var out wrp.Message
			require.NoError(t, got[0].To(&out))
			assert.Equal(t, msg, out)
		})
	}
}
