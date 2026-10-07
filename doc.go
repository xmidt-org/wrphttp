// SPDX-FileCopyrightText: 2022 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

/*
Package wrphttp translates between WRP messages and HTTP requests and
responses.  The Encoder turns messages into an HTTP request or response body,
and DecodeRequest, DecodeResponse and DecodeFromParts turn them back into
messages.

# Forms

A WRP message is carried over HTTP in one of these forms:

  - A body: the whole message encoded as msgpack (application/msgpack) or
    JSON (application/json).
  - Several messages in one body: JSONL (application/jsonl), MsgpackL
    (application/msgpackl), or multipart/mixed with one message per part.
  - Header form: the payload is the body and the other fields are headers,
    such as X-Xmidt-Message-Type and X-Xmidt-Source.  The Content-Type is the
    payload's type.

When decoding, the form is chosen this way:

  - If a message type header is present, the message is in header form,
    whatever the Content-Type.  This matches wrp-go/v3.
  - Otherwise the Content-Type decides.  A missing Content-Type means
    msgpack, and the deprecated application/wrp also means msgpack.

In header form each Headers entry is one X-Xmidt-Headers line.  An
intermediary may fold those lines into one comma separated line (RFC 9110
§5.3), so a line is split back into entries at each comma that is followed by
another "name:" entry.  A comma inside an entry's value, as in a tracestate,
is kept.  wrp-go/v3 splits at every comma.

# Working with wrp-go/v3

Every form github.com/xmidt-org/wrp-go/v3/wrphttp sends is decoded, subject to
validation (see below).  To send
messages a wrp-go/v3 peer can read, use the default options or AsJSON, and do
not use:

  - more than one message per request or response
  - AsJSONL or AsMsgpackL
  - EncodeGzip, EncodeDeflate or EncodeZlib
  - AsOctetStream with a style other than the default "X-Webpa"

A wrp-go/v3 server only reads header form when it uses
DecodeEntityFromSources(_, true).

wrp-go/v3 accepts any field on any message type.  The standard wrp validation
does not, so messages from wrp-go/v3 may need wrp.NoStandardValidation().
*/
package wrphttp
