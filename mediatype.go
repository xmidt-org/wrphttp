// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrphttp

type mediaType string

const (
	mtJSON        mediaType = "application/wrp+json"
	mtMsgpack     mediaType = "application/wrp+msgpack"
	mtOctetStream mediaType = "application/wrp+octet-stream"
	mtJSONL       mediaType = "application/wrp+jsonl"
	mtMsgpackL    mediaType = "application/wrp+msgpackl"
)

func (mt mediaType) String() string {
	return string(mt)
}
