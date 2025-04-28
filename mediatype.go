// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrphttp

import (
	"fmt"
	"mime"
	"sort"
	"strings"
)

type mediaType string

const (
	MEDIA_TYPE_JSON         = "application/json"
	MEDIA_TYPE_MSGPACK      = "application/msgpack"
	MEDIA_TYPE_OCTET_STREAM = "application/octet-stream"
	MEDIA_TYPE_JSONL        = "application/jsonl"
	MEDIA_TYPE_MSGPACKL     = "application/msgpackl"

	// These are the styles that are supported for octet-stream
	MEDIA_TYPE_OCTET_STREAM_X_XMIDT_STYLE = "application/octet-stream; style=x-xmidt"
	MEDIA_TYPE_OCTET_STREAM_X_MIDT_STYLE  = "application/octet-stream; style=x-midt"
	MEDIA_TYPE_OCTET_STREAM_XMIDT_STYLE   = "application/octet-stream; style=xmidt"
	MEDIA_TYPE_OCTET_STREAM_WEBPA_STYLE   = "application/octet-stream; style=x-webpa"

	mtUnknown           mediaType = ""
	mtJSON              mediaType = MEDIA_TYPE_JSON
	mtMsgpack           mediaType = MEDIA_TYPE_MSGPACK
	mtOctetStream       mediaType = MEDIA_TYPE_OCTET_STREAM
	mtOctetStreamXXmidt mediaType = MEDIA_TYPE_OCTET_STREAM_X_XMIDT_STYLE
	mtOctetStreamXMidt  mediaType = MEDIA_TYPE_OCTET_STREAM_X_MIDT_STYLE
	mtOctetStreamXWebpa mediaType = MEDIA_TYPE_OCTET_STREAM_WEBPA_STYLE
	mtOctetStreamXmidt  mediaType = MEDIA_TYPE_OCTET_STREAM_XMIDT_STYLE
	mtJSONL             mediaType = MEDIA_TYPE_JSONL
	mtMsgpackL          mediaType = MEDIA_TYPE_MSGPACKL
)

func (mt mediaType) String() string {
	return string(mt)
}

// AllMediaTypes returns a list of all the media types supported by the encoder.
// This allows new formats to be added in the future without breaking existing
// code.  The list is not guaranteed to be in any particular order.
func AllMediaTypes() []string {
	keys := make([]string, 0, len(mtFromString))
	for k := range mtFromString {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

var (
	mtFromString = map[string]mediaType{
		MEDIA_TYPE_JSON:                       mtJSON,
		MEDIA_TYPE_MSGPACK:                    mtMsgpack,
		MEDIA_TYPE_OCTET_STREAM:               mtOctetStream,
		MEDIA_TYPE_JSONL:                      mtJSONL,
		MEDIA_TYPE_MSGPACKL:                   mtMsgpackL,
		MEDIA_TYPE_OCTET_STREAM_X_XMIDT_STYLE: mtOctetStreamXXmidt,
		MEDIA_TYPE_OCTET_STREAM_X_MIDT_STYLE:  mtOctetStreamXMidt,
		MEDIA_TYPE_OCTET_STREAM_XMIDT_STYLE:   mtOctetStreamXmidt,
		MEDIA_TYPE_OCTET_STREAM_WEBPA_STYLE:   mtOctetStreamXWebpa,
	}
)

func toMediaType(mt, style string) (mediaType, error) {
	got, ok := mtFromString[mt]
	if !ok {
		return mtUnknown, fmt.Errorf("unsupported media type: %s", mt)
	}

	if got != mtOctetStream {
		return got, nil
	}

	switch style {
	case "":
		return mtOctetStream, nil
	case styleXXmidt:
		return mtOctetStreamXXmidt, nil
	case styleXMidt:
		return mtOctetStreamXMidt, nil
	case styleXmidt:
		return mtOctetStreamXmidt, nil
	case styleXWebpa:
		return mtOctetStreamXWebpa, nil
	default:
		allowed := fmt.Sprintf(
			"%q, %q, %q, %q, %q",
			"", styleXXmidt, styleXMidt, styleXmidt, styleXWebpa)
		return mtUnknown, fmt.Errorf(
			"unsupported octet-stream style: %s, must be one of %s",
			style, allowed)
	}
}

func toMediaTypeFromMime(s string) (mediaType, error) {
	mt, params, err := mime.ParseMediaType(strings.TrimSpace(s))
	if err != nil {
		return mtUnknown, err
	}

	return toMediaType(mt, params["style"])
}
