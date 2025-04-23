// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrphttp

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/tinylib/msgp/msgp"
	"github.com/xmidt-org/wrp-go/v5"
)

// DecodeRequest converts an http.Request into the provided wrp messages if
// applicable.  This will handle any of the valid forms the encoder can produce.
func DecodeRequest(req *http.Request, validators ...wrp.Processor) ([]wrp.Union, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	ct, _, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(ct, "multipart/") {
		return fromPart(req.Header, req.Body, validators...)
	}

	mr, err := req.MultipartReader()
	if err != nil {
		return nil, err
	}

	var rv []wrp.Union
	for {
		part, err := mr.NextPart()
		if err == io.EOF { // nolint: errorlint
			return rv, nil
		}
		if err != nil {
			return nil, err
		}

		msgs, err := fromPart(http.Header(part.Header), part, validators...)
		if err != nil {
			return nil, err
		}
		rv = append(rv, msgs...)
	}
}

// DecodeResponse converts an http.Response into the provided wrp messages if
// applicable.  This will handle any of the valid forms the encoder can produce.
func DecodeResponse(resp *http.Response, validators ...wrp.Processor) ([]wrp.Union, error) {
	if resp == nil {
		return nil, fmt.Errorf("response is nil")
	}

	return DecodeFromParts(resp.Header, resp.Body, validators...)
}

// DecodeFromParts converts an http.Header and io.ReadCloser into the provided wrp
// messages if applicable.  This will handle any of the valid forms the encoder
// can produce.
func DecodeFromParts(headers http.Header, body io.ReadCloser, validators ...wrp.Processor) ([]wrp.Union, error) {
	mediaType, params, err := mime.ParseMediaType(headers.Get("Content-Type"))
	if err != nil {
		body.Close()
		return nil, fmt.Errorf("invalid Content-Type: %w", err)
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		return fromPart(headers, body, validators...)
	}

	defer body.Close()

	boundary := params["boundary"]
	if boundary == "" {
		return nil, fmt.Errorf("missing boundary in Content-Type: %s", headers.Get("Content-Type"))
	}
	if mediaType != "multipart/mixed" {
		return nil, fmt.Errorf("unsupported media type: %s", mediaType)
	}

	mr := multipart.NewReader(body, boundary)

	var rv []wrp.Union
	for {
		part, err := mr.NextPart()
		if err == io.EOF { // nolint: errorlint
			return rv, nil
		}
		if err != nil {
			return nil, err
		}
		defer part.Close()

		msgs, err := fromPart(http.Header(part.Header), part, validators...)
		if err != nil {
			return nil, err
		}
		rv = append(rv, msgs...)
	}
}

func handleEncoding(h http.Header, body io.ReadCloser) (io.ReadCloser, error) {
	et := h.Get("Content-Encoding")
	switch et {
	case "gzip":
		return gzip.NewReader(body)
	case "deflate":
		return flate.NewReader(body), nil
	case "zlib":
		return zlib.NewReader(body)
	case "identity", "":
		return body, nil
	default:
	}
	return nil, fmt.Errorf("unsupported content encoding: %s", et)
}

func fromPart(h http.Header, body io.ReadCloser, validators ...wrp.Processor) ([]wrp.Union, error) {
	var err error

	if body != nil {
		defer body.Close()
	}

	body, err = handleEncoding(h, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		defer body.Close()
	}

	ct, _, err := mime.ParseMediaType(h.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("invalid Content-Type: %w", err)
	}
	ct = strings.TrimPrefix(ct, "multipart/")

	switch mediaType(ct) {
	case mtJSON:
		return fromFormat(wrp.JSON, body, validators...)
	case mtMsgpack:
		return fromFormat(wrp.Msgpack, body, validators...)
	case mtOctetStream:
		return fromOctetStream(h, body, validators...)
	case mtJSONL:
		return fromJSONL(body, validators...)
	case mtMsgpackL:
		return fromMsgpackL(body, validators...)
	}

	return nil, fmt.Errorf("unsupported media type: %s", ct)
}

func fromFormat(f wrp.Format, body io.ReadCloser, validators ...wrp.Processor) ([]wrp.Union, error) {
	var msg wrp.Message
	if err := f.Decoder(body).Decode(&msg, validators...); err != nil {
		return nil, err
	}
	return []wrp.Union{&msg}, nil
}

func fromOctetStream(h http.Header, body io.ReadCloser, validators ...wrp.Processor) ([]wrp.Union, error) {
	msg, err := fromHeaders(h, body, validators...)
	if err != nil {
		return nil, err
	}
	return []wrp.Union{msg}, nil
}

func fromJSONL(body io.ReadCloser, validators ...wrp.Processor) ([]wrp.Union, error) {
	var msgs []wrp.Union
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		var msg wrp.Message
		line := scanner.Bytes()
		if err := wrp.JSON.DecoderBytes(line).Decode(&msg, validators...); err != nil {
			return nil, err
		}
		msgs = append(msgs, &msg)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return msgs, nil
}

func fromMsgpackL(body io.ReadCloser, validators ...wrp.Processor) ([]wrp.Union, error) {
	var msgs []wrp.Union
	r := msgp.NewReader(body)
	count, err := r.ReadArrayHeader()
	if err != nil {
		return nil, err
	}
	var i uint32
	for ; i < count; i++ {
		var msg wrp.Message
		var item []byte
		item, err = r.ReadBytes(nil)
		if err == nil {
			err = wrp.Msgpack.DecoderBytes(item).Decode(&msg, validators...)
		}

		if err != nil {
			return nil, err
		}
		msgs = append(msgs, &msg)
	}
	return msgs, nil
}
