// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrphttp

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/xmidt-org/wrp-go/v5"
)

type hdr []string

func (h hdr) Get(headers http.Header) string {
	for _, key := range h {
		if val := headers.Get(key); val != "" {
			return val
		}
	}
	return ""
}

func (h hdr) WhichStyle(headers http.Header) string {
	for i, key := range h {
		if val := headers.Get(key); val != "" {
			return orderedStyles[i]
		}
	}
	return ""
}

func (h hdr) Values(headers http.Header) []string {
	var values []string
	for _, key := range h {
		if val := headers.Values(key); len(val) > 0 {
			values = append(values, val...)
		}
	}
	return values
}

const (
	styleXXmidt = "x-xmidt" // lowercase version of X-Xmidt
	styleXMidt  = "x-midt"  // lowercase version of X-Midt
	styleXmidt  = "xmidt"   // lowercase version of Xmidt
	styleXWebpa = "x-webpa" // lowercase version of X-Webpa
)

func (h hdr) As(s string) string {
	switch s {
	case styleXXmidt:
		return h[0]
	case styleXMidt:
		return h[1]
	case styleXmidt:
		return h[2]
	case styleXWebpa:
		if len(h) == 4 {
			return h[3]
		}
		return h[0]
	}

	return ""
}

var (
	orderedStyles = []string{styleXXmidt, styleXMidt, styleXmidt, styleXWebpa}

	// Ensure the formats are: X-Xmidt, X-Midt, Xmidt, X-Webpa
	messageTypeHeader     = hdr{"X-Xmidt-Message-Type" /*        */, "X-Midt-Message-Type" /*        */, "Xmidt-Message-Type" /*        */}
	transactionUuidHeader = hdr{"X-Xmidt-Transaction-Uuid" /*    */, "X-Midt-Transaction-Uuid" /*    */, "Xmidt-Transaction-Uuid" /*    */}
	statusHeader          = hdr{"X-Xmidt-Status" /*              */, "X-Midt-Status" /*              */, "Xmidt-Status" /*              */}
	rdrHeader             = hdr{"X-Xmidt-Request-Delivery-Response", "X-Midt-Request-Delivery-Response", "Xmidt-Request-Delivery-Response"}
	pathHeader            = hdr{"X-Xmidt-Path" /*                */, "X-Midt-Path" /*                */, "Xmidt-Path" /*                */}
	sourceHeader          = hdr{"X-Xmidt-Source" /*              */, "X-Midt-Source" /*              */, "Xmidt-Source" /*              */}
	destinationHeader     = hdr{"X-Xmidt-Destination" /*         */, "X-Midt-Destination" /*         */, "Xmidt-Destination" /*         */, "X-Webpa-Device-Name"}
	acceptHeader          = hdr{"X-Xmidt-Accept" /*              */, "X-Midt-Accept" /*              */, "Xmidt-Accept" /*              */}
	metadataHeader        = hdr{"X-Xmidt-Metadata" /*            */, "X-Midt-Metadata" /*            */, "Xmidt-Metadata" /*            */}
	partnerIdHeader       = hdr{"X-Xmidt-Partner-Id" /*          */, "X-Midt-Partner-Id" /*          */, "Xmidt-Partner-Id" /*          */}
	sessionIdHeader       = hdr{"X-Xmidt-Session-Id" /*          */, "X-Midt-Session-Id" /*          */, "Xmidt-Session-Id" /*          */}
	headersHeader         = hdr{"X-Xmidt-Headers" /*             */, "X-Midt-Headers" /*             */, "Xmidt-Headers" /*             */}
	serviceNameHeader     = hdr{"X-Xmidt-Service-Name" /*        */, "X-Midt-Service-Name" /*        */, "Xmidt-Service-Name" /*        */}
	urlHeader             = hdr{"X-Xmidt-Url" /*                 */, "X-Midt-Url" /*                 */, "Xmidt-Url" /*                 */}
	contentTypeHeader     = hdr{"X-Xmidt-Content-Type" /*        */, "X-Midt-Content-Type" /*        */, "Xmidt-Content-Type" /*        */}
)

// legacyMessageTypeHeader is the deprecated message type header that
// wrp-go/v3 still accepts.  It does not follow the naming of any style, so it
// is only ever read.
const legacyMessageTypeHeader = "X-Midt-Msg-Type"

func getMessageType(headers http.Header) string {
	if msgType := messageTypeHeader.Get(headers); msgType != "" {
		return msgType
	}
	return headers.Get(legacyMessageTypeHeader)
}

// isHeaderForm reports whether the WRP fields are carried in the headers.
func isHeaderForm(headers http.Header) bool {
	return getMessageType(headers) != ""
}

// payloadContentType returns the media type of a header form payload when it
// is not explicitly provided.  Like wrp-go/v3, the Content-Type describes the
// payload.  An octet-stream Content-Type is treated as unspecified since it is
// both what an empty content type means and the envelope this package uses.
func payloadContentType(headers http.Header) string {
	ct := headers.Get("Content-Type")
	if mt, _, err := mime.ParseMediaType(ct); err == nil && mt == MEDIA_TYPE_OCTET_STREAM {
		return ""
	}
	return ct
}

func toHeadersForm(msg wrp.Union, typ string, validators ...wrp.Processor) (http.Header, []byte, error) {
	headers := make(http.Header)

	var out wrp.Message
	if err := msg.To(&out, validators...); err != nil {
		return nil, nil, err
	}

	h := wrpHeader{headers: headers, typ: typ}

	headers.Set(messageTypeHeader.As(typ), out.MsgType().FriendlyName())

	h.toIntPtrHeader(statusHeader, out.Status, headers)
	h.toIntPtrHeader(rdrHeader, out.RequestDeliveryResponse, headers)
	h.toStringHeader(transactionUuidHeader, out.TransactionUUID, headers)
	h.toStringHeader(pathHeader, out.Path, headers)
	h.toStringHeader(sourceHeader, out.Source, headers)
	h.toStringHeader(destinationHeader, out.Destination, headers)
	h.toStringHeader(acceptHeader, out.Accept, headers)
	h.toStringHeader(sessionIdHeader, out.SessionID, headers)
	h.toStringHeader(serviceNameHeader, out.ServiceName, headers)
	h.toStringHeader(urlHeader, out.URL, headers)
	h.toStringHeader(contentTypeHeader, out.ContentType, headers)
	for k, v := range out.Metadata {
		headers.Add(metadataHeader.As(typ), k+"="+v)
	}
	partners := strings.Join(out.PartnerIDs, ",")
	if partners != "" {
		headers.Set(partnerIdHeader.As(typ), partners)
	}
	if out.Headers != nil {
		for _, v := range out.Headers {
			if v != "" {
				headers.Add(headersHeader.As(typ), v)
			}
		}
	}

	return headers, out.Payload, nil
}

func fromHeaders(headers http.Header, body io.ReadCloser, validators ...wrp.Processor) (wrp.Union, error) {
	var msg wrp.Message

	if msgType := getMessageType(headers); msgType != "" {
		msg.Type = wrp.StringToMessageType(msgType)
	}

	h := wrpHeader{headers: headers}

	h.readString(transactionUuidHeader, &msg.TransactionUUID)
	h.readInt(statusHeader, &msg.Status)
	h.readInt(rdrHeader, &msg.RequestDeliveryResponse)
	h.readString(pathHeader, &msg.Path)
	h.readString(sourceHeader, &msg.Source)
	h.readString(destinationHeader, &msg.Destination)
	h.readString(acceptHeader, &msg.Accept)
	h.readString(sessionIdHeader, &msg.SessionID)
	h.readString(serviceNameHeader, &msg.ServiceName)
	h.readString(urlHeader, &msg.URL)
	h.readStrings(partnerIdHeader, &msg.PartnerIDs)
	h.readHashmap(metadataHeader, &msg.Metadata)
	h.readHeaders(headersHeader, &msg.Headers)
	h.readString(contentTypeHeader, &msg.ContentType)
	if msg.ContentType == "" {
		msg.ContentType = payloadContentType(headers)
	}

	if body != nil {
		payload, err := io.ReadAll(body)
		defer body.Close()

		if err != nil {
			return nil, fmt.Errorf("failed to read body: %w", err)
		}
		msg.Payload = payload
	}

	if err := msg.Validate(validators...); err != nil {
		return nil, err
	}

	return &msg, nil
}

type wrpHeader struct {
	headers http.Header
	typ     string
}

func (h wrpHeader) toStringHeader(key hdr, value string, headers http.Header) {
	if value != "" {
		headers.Set(key.As(h.typ), value)
	}
}

func (h wrpHeader) toIntPtrHeader(key hdr, value *int64, headers http.Header) {
	if value != nil {
		headers.Set(key.As(h.typ), fmt.Sprintf("%d", *value))
	}
}

func (h wrpHeader) readString(key hdr, target *string) {
	if val := key.Get(h.headers); val != "" {
		*target = val
	}
}

func (h wrpHeader) readStrings(key hdr, target *[]string) {
	if val := key.Values(h.headers); len(val) > 0 {
		list := make([]string, 0, len(val))
		for _, p := range val {
			items := strings.Split(p, ",")
			for _, item := range items {
				item = strings.TrimSpace(item)
				if item != "" {
					list = append(list, item)
				}
			}
		}
		*target = list
	}
}

func (h wrpHeader) readInt(key hdr, target **int64) {
	if val := key.Get(h.headers); val != "" {
		v, err := strconv.ParseInt(val, 10, 64)
		if err == nil {
			if *target == nil {
				*target = new(int64)
			}
			**target = v
		}
	}
}

// readHashmap reads metadata the way wrp-go/v3 does: each header value is a
// comma separated list of key=value pairs.
func (h wrpHeader) readHashmap(key hdr, target *map[string]string) {
	if hashmap := key.Values(h.headers); len(hashmap) > 0 {
		rv := make(map[string]string)
		for _, line := range hashmap {
			for _, pair := range strings.Split(line, ",") {
				k, v, _ := strings.Cut(pair, "=")
				rv[strings.TrimSpace(k)] = v
			}
		}
		*target = rv
	}
}

// readHeaders reads the Headers field.  A sender writes one entry per header
// line, but an intermediary may fold repeated lines into one comma separated
// line (RFC 9110 §5.3), so each line is split back into entries.
func (h wrpHeader) readHeaders(key hdr, target *[]string) {
	if array := key.Values(h.headers); len(array) > 0 {
		rv := make([]string, 0, len(array))
		for _, line := range array {
			rv = append(rv, splitHeaderEntries(line)...)
		}
		*target = rv
	}
}

// splitHeaderEntries splits a folded Headers line into its entries.
//
// An entry is itself an HTTP style "Name: value" header and its value may
// contain commas, as a tracestate does, so a comma only ends an entry when
// what follows it starts another one: a header name and a colon.  Given
//
//	traceparent: a, tracestate: b=1,c=2
//
// the result is "traceparent: a" and "tracestate: b=1,c=2".  wrp-go/v3
// splits at every comma, which breaks the tracestate into two entries.
//
// A value that contains a comma followed by a "name:" of its own is split
// wrongly; such a value can't be told apart from two entries.
func splitHeaderEntries(line string) []string {
	var entries []string
	start := 0
	for i := 0; i < len(line); i++ {
		if line[i] == ',' && isEntryBoundary(line[i+1:]) {
			entries = appendHeaderEntry(entries, line[start:i])
			start = i + 1
		}
	}
	return appendHeaderEntry(entries, line[start:])
}

// ows is optional whitespace (RFC 9110 §5.6.3), what may surround an entry.
const ows = " \t"

func appendHeaderEntry(entries []string, entry string) []string {
	if entry = strings.Trim(entry, ows); entry != "" {
		entries = append(entries, entry)
	}
	return entries
}

// isEntryBoundary reports whether the comma before rest ends an entry: rest
// starts a new "name:" entry, or is empty or another comma, an empty list
// element that RFC 9110 §5.6.1 says to ignore.
func isEntryBoundary(rest string) bool {
	s := strings.TrimLeft(rest, ows)
	if s == "" || s[0] == ',' {
		return true
	}
	i := 0
	for i < len(s) && isTokenChar(s[i]) {
		i++
	}
	return i > 0 && i < len(s) && s[i] == ':'
}

// isTokenChar reports whether c is a tchar (RFC 9110 §5.6.2), the characters
// allowed in a header field name.
func isTokenChar(c byte) bool {
	switch {
	case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		return true
	}
	return strings.IndexByte("!#$%&'*+-.^_`|~", c) >= 0
}
