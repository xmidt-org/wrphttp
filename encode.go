// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrphttp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/textproto"

	"github.com/tinylib/msgp/msgp"
	"github.com/xmidt-org/wrp-go/v5"
)

type compressor func(io.Writer) (io.WriteCloser, error)

// Encoder contains the options used for encoding new http.Request and http.Response
// objects.  The Encoder is not safe for concurrent use.
type Encoder struct {
	mt         mediaType
	compressor compressor
	encoding   string
	validator  []wrp.Processor
	style      string
	maxItems   int
}

// Option is a functional option for configuring the Encoder.  The options are
// applied in the order they are provided.
type Option interface {
	apply(*Encoder) error
}

// NewEncoder creates a new Encoder with the provided options.  The options are
// applied in the order they are provided.  If no options are provided, the
// default options are used.  The Encoder is not safe for concurrent use.
//
//	The default options are:
//	 - AsMsgpack()
//	 - EncodeNoCompression()
//	 - WithMaxItemsPerChunk(0)
func NewEncoder(opts ...Option) (*Encoder, error) {
	var encoder Encoder

	defaults := []Option{
		AsMsgpack(),
		EncodeNoCompression(),
		WithMaxItemsPerChunk(0),
	}

	opts = append(defaults, opts...)

	for _, opt := range opts {
		if opt != nil {
			if err := opt.apply(&encoder); err != nil {
				return nil, err
			}
		}
	}

	return &encoder, nil
}

// NewRequest creates a new http.Request with the provided method, URL, and
// messages.  The messages are encoded using the Encoder's media type and
// compression.  The request is not sent and the body is not closed.
func (e *Encoder) NewRequest(method, url string, msgs ...wrp.Union) (*http.Request, error) {
	return e.NewRequestWithContext(context.Background(), method, url, msgs...)
}

// NewRequestWithContext creates a new http.Request with the provided context
// in addition to the method, URL, and messages.  The messages are encoded
// using the Encoder's media type and compression.  The request is not sent
// and the body is not closed.
func (e *Encoder) NewRequestWithContext(ctx context.Context, method, url string, msgs ...wrp.Union) (*http.Request, error) {
	h, body, err := e.ToParts(msgs...)
	if err != nil {
		return nil, err
	}
	// Construct the HTTP request with the pipe reader as the body
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	maps.Copy(req.Header, h)
	return req, nil
}

// ToParts encodes the provided messages into a mixed response body that can be
// used in a http.Response.  The messages are encoded using the Encoder's
// media type and compression.
func (e *Encoder) ToParts(msgs ...wrp.Union) (http.Header, io.Reader, error) {
	if len(msgs) == 0 {
		return nil, nil, fmt.Errorf("no messages provided")
	}

	pr, pw := io.Pipe()

	var boundary string
	headers := e.getHeaders()

	switch e.mt {
	case mtJSON:
		boundary = e.asFormat(wrp.JSON, pw, msgs...)
	case mtMsgpack:
		boundary = e.asFormat(wrp.Msgpack, pw, msgs...)
	case mtOctetStream:
		var err error
		headers, boundary, err = e.asOctetStream(pw, msgs...)
		if err != nil {
			return nil, nil, err
		}
	case mtMsgpackL:
		boundary = e.asMsgpackL(pw, msgs...)
	case mtJSONL:
		boundary = e.asJSONL(pw, msgs...)
	}

	if boundary != "" {
		headers.Set("Content-Type", fmt.Sprintf("multipart/mixed; boundary=%s", boundary))
	}

	return headers, pr, nil
}

func (e *Encoder) asFormat(f wrp.Format, pw *io.PipeWriter, msgs ...wrp.Union) string {
	if len(msgs) == 1 {
		e.asFormatSingle(f, pw, msgs...)
		return ""
	}
	return e.asFormatMultiPart(f, pw, msgs...)
}

func (e *Encoder) asFormatSingle(f wrp.Format, pw *io.PipeWriter, msgs ...wrp.Union) {
	go func() {
		// Wrap the pipe writer with the compressor
		cw, err := e.compressor(pw)
		if err == nil {
			err = f.Encoder(cw).Encode(msgs[0], e.validator...)
			cw.Close()
		}

		if err != nil {
			pw.CloseWithError(err)
		}

		pw.Close()
	}()
}

func (e *Encoder) asFormatMultiPart(f wrp.Format, pw *io.PipeWriter, msgs ...wrp.Union) string {
	// Multiple messages: use multipart encoding
	mw := multipart.NewWriter(pw)

	go func() {
		defer func() {
			if err := mw.Close(); err != nil {
				pw.CloseWithError(err)
			} else {
				pw.Close()
			}
		}()

		header := textproto.MIMEHeader(e.getHeaders())

		for _, msg := range msgs {
			part, err := mw.CreatePart(header)
			if err != nil {
				pw.CloseWithError(err)
				return
			}

			// Wrap the pipe writer with the compressor
			cw, err := e.compressor(part)
			if err == nil {
				err = f.Encoder(cw).Encode(msg, e.validator...)
				cw.Close()
			}

			if err != nil {
				pw.CloseWithError(err)
				return
			}
		}
	}()

	return mw.Boundary()
}

func (e *Encoder) asOctetStream(pw *io.PipeWriter, msgs ...wrp.Union) (http.Header, string, error) {
	if len(msgs) == 1 {
		h, err := e.asOctetStreamSingle(pw, msgs[0])
		if err != nil {
			return nil, "", err
		}
		return h, "", nil
	}

	return e.asOctetStreamMultiPart(pw, msgs...)
}

func (e *Encoder) asOctetStreamSingle(pw *io.PipeWriter, msgs ...wrp.Union) (http.Header, error) {
	headers, payload, err := toHeadersForm(msgs[0], e.style, e.validator...)
	if err != nil {
		return nil, err
	}

	go func() {
		// Wrap the pipe writer with the compressor
		cw, err := e.compressor(pw)
		if err == nil {
			_, err = cw.Write(payload)
			cw.Close()
		}

		if err != nil {
			pw.CloseWithError(err)
			return
		}

		pw.Close()
	}()

	return e.getHeaders(headers), nil
}

func (e *Encoder) asOctetStreamMultiPart(pw *io.PipeWriter, msgs ...wrp.Union) (http.Header, string, error) {
	// Multiple messages: use multipart encoding
	mw := multipart.NewWriter(pw)

	go func() {
		defer func() {
			if err := mw.Close(); err != nil {
				pw.CloseWithError(err)
			} else {
				pw.Close()
			}
		}()

		for _, msg := range msgs {
			headers, payload, err := toHeadersForm(msg, e.style, e.validator...)
			if err == nil {
				var part io.Writer
				headers = e.getHeaders(headers)
				part, err = mw.CreatePart(textproto.MIMEHeader(headers))
				if err == nil {
					var cw io.WriteCloser
					// Wrap the pipe writer with the compressor
					cw, err = e.compressor(part)
					if err == nil {
						_, err = cw.Write(payload)
						cw.Close()
					}
				}
			}

			if err != nil {
				pw.CloseWithError(err)
				return
			}
		}
	}()

	return e.getHeaders(), mw.Boundary(), nil
}

func (e *Encoder) asMsgpackL(pw *io.PipeWriter, msgs ...wrp.Union) string {
	if e.maxItems < 1 || len(msgs) <= e.maxItems {
		e.asMsgpackLSingle(pw, msgs...)
		return ""
	}
	return e.chunkedMultipart(pw,
		func(w io.Writer, msgs []wrp.Union) error {
			return e.asMsgpackLArray(w, msgs...)
		},
		msgs...)
}

func (e *Encoder) asMsgpackLSingle(pw *io.PipeWriter, msgs ...wrp.Union) {
	go func() {
		// Wrap the pipe writer with the compressor
		cw, err := e.compressor(pw)
		if err == nil {
			err = e.asMsgpackLArray(cw, msgs...)
			cw.Close()
		}
		if err != nil {
			pw.CloseWithError(err)
		}

		pw.Close()
	}()
}

func (e *Encoder) asMsgpackLArray(w io.Writer, msgs ...wrp.Union) error {
	wr := msgp.NewWriter(w)
	if err := wr.WriteArrayHeader(uint32(len(msgs))); err != nil { // nolint: gosec
		return err
	}

	for _, msg := range msgs {
		var item bytes.Buffer
		err := wrp.Msgpack.Encoder(&item).Encode(msg, e.validator...)
		if err == nil {
			err = wr.WriteBytes(item.Bytes())
		}

		if err != nil {
			return err
		}
	}

	_ = wr.Flush()

	return nil
}

type encoderPartFunc func(w io.Writer, msgs []wrp.Union) error

func (e *Encoder) chunkedMultipart(pw *io.PipeWriter, fn encoderPartFunc, msgs ...wrp.Union) string {
	// Multiple messages: use multipart encoding
	mw := multipart.NewWriter(pw)
	go func() {
		defer func() {
			if err := mw.Close(); err != nil {
				pw.CloseWithError(err)
			} else {
				pw.Close()
			}
		}()
		header := textproto.MIMEHeader(e.getHeaders())

		items := chunked{
			list:     msgs,
			perChunk: e.maxItems,
		}
		for {
			msgs := items.Next()
			if msgs == nil {
				return
			}

			part, err := mw.CreatePart(header)
			if err == nil {
				var cw io.WriteCloser
				cw, err = e.compressor(part)
				if err == nil {
					err = fn(cw, msgs)
					cw.Close()
				}
			}
			if err != nil {
				pw.CloseWithError(err)
				return
			}
		}
	}()

	return mw.Boundary()
}

func (e *Encoder) asJSONL(pw *io.PipeWriter, msgs ...wrp.Union) string {
	if e.maxItems < 1 || len(msgs) <= e.maxItems {
		e.asJSONLSingle(pw, msgs...)
		return ""
	}
	return e.chunkedMultipart(pw,
		func(w io.Writer, msgs []wrp.Union) error {
			return e.asJSONLArray(w, msgs...)
		}, msgs...)
}

func (e *Encoder) asJSONLArray(w io.Writer, msgs ...wrp.Union) error {
	for _, msg := range msgs {
		if err := wrp.JSON.Encoder(w).Encode(msg, e.validator...); err != nil {
			return err
		}
	}

	return nil
}

func (e *Encoder) asJSONLSingle(pw *io.PipeWriter, msgs ...wrp.Union) {
	go func() {
		// Wrap the pipe writer with the compressor
		cw, err := e.compressor(pw)
		if err == nil {
			err = e.asJSONLArray(cw, msgs...)
			cw.Close()
		}
		if err != nil {
			pw.CloseWithError(err)
			return
		}

		pw.Close()
	}()
}

func (e *Encoder) getHeaders(h ...http.Header) http.Header {
	h = append(h, make(http.Header, 2))
	h[0].Set("Content-Type", e.mt.String())
	if e.encoding != "" && e.encoding != "identity" {
		h[0].Set("Content-Encoding", e.encoding)
	}
	return h[0]
}

type chunked struct {
	list     []wrp.Union
	perChunk int
	current  int
}

func (c *chunked) Next() []wrp.Union {
	if c.current >= len(c.list) {
		return nil
	}

	end := c.current + c.perChunk
	if end > len(c.list) {
		end = len(c.list)
	}

	chunk := c.list[c.current:end]
	c.current = end

	return chunk
}
