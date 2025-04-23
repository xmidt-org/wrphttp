// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrphttp

import (
	"fmt"
	"io"
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
	styleXXmidt = "X-Xmidt"
	styleXMidt  = "X-Midt"
	styleXmidt  = "Xmidt"
	styleXWebpa = "X-Webpa"
)

func (h hdr) As(s string) string {
	switch s {
	case "X-Xmidt":
		return h[0]
	case "X-Midt":
		return h[1]
	case "Xmidt":
		return h[2]
	case "X-Webpa":
		if len(h) == 4 {
			return h[3]
		}
		return h[0]
	}

	return ""
}

var (
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
)

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
	if out.Metadata != nil {
		for k, v := range out.Metadata {
			if v != "" {
				headers.Add(metadataHeader.As(typ), fmt.Sprintf("%s:%s", k, v))
			}
		}
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

	if msgType := messageTypeHeader.Get(headers); msgType != "" {
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

func (h wrpHeader) readHashmap(key hdr, target *map[string]string) {
	if hashmap := key.Values(h.headers); len(hashmap) > 0 {
		rv := make(map[string]string)
		for _, m := range hashmap {
			pair := strings.SplitN(m, ":", 2)
			if len(pair) == 2 {
				key := strings.TrimSpace(pair[0])
				value := strings.TrimSpace(pair[1])
				rv[key] = value
			}
		}
		*target = rv
	}
}

func (h wrpHeader) readHeaders(key hdr, target *[]string) {
	if array := key.Values(h.headers); len(array) > 0 {
		rv := make([]string, 0, len(array))
		for _, item := range array {
			if item != "" {
				rv = append(rv, item)
			}
		}
		*target = rv
	}
}
