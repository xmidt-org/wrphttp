// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrphttp

import (
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"

	"github.com/xmidt-org/wrp-go/v5"
)

type optionFuncErr func(*Encoder) error

func (f optionFuncErr) apply(e *Encoder) error {
	return f(e)
}

type optionFunc func(*Encoder)

func (f optionFunc) apply(e *Encoder) error {
	f(e)
	return nil
}

// AsJSON sets the encoder to use JSON encoding for WRP messages.  A single
// message is encoded with the body as the wrp.Message encoded as JSON with
// the Content-Type set to "application/wrp+json".
//
// If multiple messages are provided, a multipart message is created with each
// message as a separate part.  The Content-Type of the message is set to
// "application/wrp+json".
func AsJSON() Option {
	return optionFunc(func(e *Encoder) {
		e.mt = mtJSON
	})
}

// AsMsgpack sets the encoder to use Msgpack encoding for WRP messages.  A single
// message is encoded with the body as the wrp.Message encoded as Msgpack with
// the Content-Type set to "application/wrp+msgpack".
//
// If multiple messages are provided, a multipart message is created with each
// message as a separate part.  The Content-Type of the message is set to
// "application/wrp+msgpack".
func AsMsgpack() Option {
	return optionFunc(func(e *Encoder) {
		e.mt = mtMsgpack
	})
}

// AsOctetStream sets the encoder to use octet-stream encoding for WRP messages.
// This is useful for raw payloads without any encoding and the additional
// wrp fields as headers.  The optional style parameter can be used to specify
// the style of the header.
//
// The valid styles are:
//   - "X-Xmidt"
//   - "X-Midt"
//   - "Xmidt"
//   - "X-Webpa" default & best for backward compatibility
//
// The Content-Type of the message is set to "application/wrp+octet-stream".
// If multiple messages are provided, a multipart message is created with
// each message as a separate part.  The Content-Type of the message is set to
// "application/wrp+octet-stream".
func AsOctetStream(headerStyle ...string) Option {
	return optionFuncErr(func(e *Encoder) error {
		headerStyle = append(headerStyle, styleXWebpa)
		switch headerStyle[0] {
		case styleXXmidt, styleXMidt, styleXmidt, styleXWebpa:
			// valid styles
		default:
			return fmt.Errorf("invalid style %q, must be one of %q, %q, %q, %q",
				headerStyle, styleXXmidt, styleXMidt, styleXmidt, styleXWebpa)
		}

		e.mt = mtOctetStream
		e.style = headerStyle[0]
		return nil
	})
}

// AsJSONL sets the encoder to use JSONL encoding for WRP messages.  All provided
// messages are encoded as a single JSONL document up until the MaxItemsPerChunk()
// limit is reached.  If the limit is reached, a multipart message is created with
// each array of messages as a separate part.  The Content-Type of each part is set to
// "application/wrp+jsonl".
func AsJSONL() Option {
	return optionFunc(func(e *Encoder) {
		e.mt = mtJSONL
	})
}

// AsMsgpackL sets the encoder to use MsgpackL encoding for WRP messages.  All provided
// messages are encoded as a single MsgpackL document up until the MaxItemsPerChunk()
// limit is reached.  If the limit is reached, a multipart message is created with
// each array of messages as a separate part.  The Content-Type of each part is set to
// "application/wrp+msgpackl".
func AsMsgpackL() Option {
	return optionFunc(func(e *Encoder) {
		e.mt = mtMsgpackL
	})
}

// EncodeGzip uses the gzip compressor with the specified compression level.
func EncodeGzip(level ...int) Option {
	return optionFunc(func(e *Encoder) {
		level = append(level, gzip.DefaultCompression)
		e.compressor = func(w io.Writer) (io.WriteCloser, error) {
			return gzip.NewWriterLevel(w, level[0])
		}
		e.encoding = "gzip"
	})
}

// EncodeDeflate uses the deflate compressor with the specified compression level.
func EncodeDeflate(level ...int) Option {
	return optionFunc(func(e *Encoder) {
		level = append(level, flate.DefaultCompression)
		e.compressor = func(w io.Writer) (io.WriteCloser, error) {
			return flate.NewWriter(w, level[0])
		}
		e.encoding = "deflate"
	})
}

// EncodeZlib uses the zlib compressor with the specified compression level.
func EncodeZlib(level ...int) Option {
	return optionFunc(func(e *Encoder) {
		level = append(level, zlib.DefaultCompression)
		e.compressor = func(w io.Writer) (io.WriteCloser, error) {
			return zlib.NewWriterLevel(w, level[0])
		}
		e.encoding = "zlib"
	})
}

// EncodeNoCompression sets the encoder to not use any compression.  This is the
// default behavior.
func EncodeNoCompression() Option {
	return optionFunc(func(e *Encoder) {
		e.compressor = func(w io.Writer) (io.WriteCloser, error) {
			return nopWriteCloser{
				Writer: w,
			}, nil
		}
		e.encoding = "identity"
	})
}

// nopWriteCloser is a no-op implementation of io.WriteCloser
// that wraps an io.Writer and does nothing on Close.
type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error {
	return nil
}

// EncodeValidators sets the validators for the encoder.
func EncodeValidators(v ...wrp.Processor) Option {
	return optionFunc(func(e *Encoder) {
		e.validator = append(e.validator, v...)
	})
}

// WithMaxItemsPerChunk sets the maximum number of items per chunk for the encoder.
// This is useful for controlling the size of the chunks when encoding large
// payloads. The default value is 1000.
// If set to less than 0, the encoder will not chunk the payload if possible.
func WithMaxItemsPerChunk(maxItems int) Option {
	return optionFunc(func(e *Encoder) {
		if maxItems == 0 {
			maxItems = 1000
		}
		e.maxItems = maxItems
	})
}
