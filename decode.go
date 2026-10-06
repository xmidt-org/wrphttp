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

// DecodeRequest converts an http.Request into wrp messages.  It accepts every
// form the Encoder produces as well as the forms wrp-go/v3 produces.  See the
// package documentation for how the form is chosen.
//
// The standard wrp validation is applied, followed by any validators
// provided.  Include wrp.NoStandardValidation() to skip the standard
// validation.
func DecodeRequest(req *http.Request, validators ...wrp.Processor) ([]wrp.Union, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if _, _, ok := multipartType(req.Header); !ok {
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

// DecodeResponse converts an http.Response into wrp messages.  It behaves the
// same as DecodeRequest.
func DecodeResponse(resp *http.Response, validators ...wrp.Processor) ([]wrp.Union, error) {
	if resp == nil {
		return nil, fmt.Errorf("response is nil")
	}

	return DecodeFromParts(resp.Header, resp.Body, validators...)
}

// DecodeFromParts converts an http.Header and io.ReadCloser into wrp messages.
// It behaves the same as DecodeRequest.  The body is always closed.
func DecodeFromParts(headers http.Header, body io.ReadCloser, validators ...wrp.Processor) ([]wrp.Union, error) {
	mediaType, params, ok := multipartType(headers)
	if !ok {
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

// multipartType returns the parsed Content-Type if the headers describe a
// multipart body.  Header form messages are never multipart, regardless of
// the Content-Type, since that describes the payload.
func multipartType(h http.Header) (string, map[string]string, bool) {
	if isHeaderForm(h) {
		return "", nil, false
	}

	mediaType, params, err := mime.ParseMediaType(h.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		return "", nil, false
	}

	return mediaType, params, true
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

	// Like wrp-go/v3, the presence of a message type header means the WRP
	// fields are in the headers and the Content-Type describes the payload.
	if isHeaderForm(h) {
		return fromOctetStream(h, body, validators...)
	}

	// Like wrp-go/v3, a missing Content-Type means msgpack.
	ct := mtMsgpack
	if v := h.Get("Content-Type"); v != "" {
		ct, err = toMediaTypeFromMime(v)
		if err != nil {
			return nil, err
		}
	}

	switch ct {
	case mtJSON:
		return fromFormat(wrp.JSON, body, validators...)
	case mtMsgpack:
		return fromFormat(wrp.Msgpack, body, validators...)
	case mtOctetStream, mtOctetStreamXWebpa, mtOctetStreamXXmidt, mtOctetStreamXMidt, mtOctetStreamXmidt:
		return fromOctetStream(h, body, validators...)
	case mtJSONL:
		return fromJSONL(body, validators...)
	case mtMsgpackL:
		return fromMsgpackL(body, validators...)
	}

	// Unreachable.
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
