# wrphttp

wrphttp provides a library implementing helper utilities for working with WRP
messages over HTTP.

[![Build Status](https://github.com/xmidt-org/wrphttp/actions/workflows/ci.yml/badge.svg)](https://github.com/xmidt-org/wrphttp/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/xmidt-org/wrphttp/branch/main/graph/badge.svg?token=tWY4sd44iI)](https://codecov.io/gh/xmidt-org/wrphttp)
[![Apache V2 License](http://img.shields.io/badge/license-Apache%20V2-blue.svg)](https://github.com/xmidt-org/wrphttp/blob/main/LICENSE)
[![GitHub Release](https://img.shields.io/github/release/xmidt-org/wrphttp.svg)](https://github.com/xmidt-org/wrphttp/releases)
[![GoDoc](https://pkg.go.dev/badge/github.com/xmidt-org/wrphttp)](https://pkg.go.dev/github.com/xmidt-org/wrphttp)

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Examples](#examples)
- [Working with wrp-go/v3](#working-with-wrp-gov3)
- [Contributing](#contributing)

## Code of Conduct

This project and everyone participating in it are governed by the [XMiDT Code Of Conduct](https://xmidt.io/code_of_conduct/). 
By participating, you agree to this Code.

## Examples 

To use the wrphttp library, it first should be added as an import in the file you plan to use it.
Examples can be found at the top of the [GoDoc](https://godoc.org/github.com/xmidt-org/wrphttp).

## Working with wrp-go/v3

wrphttp interoperates with `github.com/xmidt-org/wrp-go/v3/wrphttp`.

**Receiving:** every form a wrp-go/v3 peer sends is decoded. wrp-go/v3
accepts any field on any message type, but the standard WRP validation
doesn't. If you receive such messages, pass `wrp.NoStandardValidation()` to
`DecodeRequest` or `DecodeResponse`.

**Sending:** the default options produce messages a wrp-go/v3 peer can read.
Don't use these with a wrp-go/v3 peer:

- more than one message per request or response
- `AsJSONL` or `AsMsgpackL`
- `EncodeGzip`, `EncodeDeflate` or `EncodeZlib`
- `AsOctetStream` with a style other than the default `X-Webpa`

A wrp-go/v3 server only reads header form (`AsOctetStream`) when it uses
`DecodeEntityFromSources(_, true)`.

The [compat](compat/) module tests all of this against wrp-go/v3.

## Contributing

Refer to [CONTRIBUTING.md](CONTRIBUTING.md).
