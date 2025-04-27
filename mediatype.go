// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrphttp

type mediaType string

const (
	MEDIA_TYPE_JSON         = "application/wrp+json"
	MEDIA_TYPE_MSGPACK      = "application/wrp+msgpack"
	MEDIA_TYPE_OCTET_STREAM = "application/wrp+octet-stream"
	MEDIA_TYPE_JSONL        = "application/wrp+jsonl"
	MEDIA_TYPE_MSGPACKL     = "application/wrp+msgpackl"

	mtJSON        mediaType = MEDIA_TYPE_JSON
	mtMsgpack     mediaType = MEDIA_TYPE_MSGPACK
	mtOctetStream mediaType = MEDIA_TYPE_OCTET_STREAM
	mtJSONL       mediaType = MEDIA_TYPE_JSONL
	mtMsgpackL    mediaType = MEDIA_TYPE_MSGPACKL
)

func (mt mediaType) String() string {
	return string(mt)
}

// AllMediaTypes returns a list of all the media types supported by the encoder.
// This allows new formats to be added in the future without breaking existing
// code.  The list is not guaranteed to be in any particular order.
func AllMediaTypes() []string {
	return []string{
		MEDIA_TYPE_MSGPACKL,
		MEDIA_TYPE_MSGPACK,
		MEDIA_TYPE_JSONL,
		MEDIA_TYPE_JSON,
		MEDIA_TYPE_OCTET_STREAM,
	}
}
