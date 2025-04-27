// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrphttp

import (
	"errors"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// NegotiateMediaType examines the headers of the request and returns the
// media type and style the request wants in response.
func NegotiateMediaType(r *http.Request) (string, error) {
	mt, err := negotiatedMediaType(r)
	if err != nil {
		return "", err
	}

	return mt.String(), nil
}

func negotiatedMediaType(r *http.Request) (mediaType, error) {
	mt, err := examineRequest(r)
	if err != nil || mt == mtUnknown {
		return mtUnknown, err
	}

	return mt, nil
}

// acceptType holds one parsed Accept media type and its parameters
type acceptType struct {
	Value  string
	Params map[string]string
	Q      float64
}

func examineContentType(r *http.Request) (mediaType, error) {
	mt, err := toMediaTypeFromMime(r.Header.Get("Content-Type"))
	if err != nil {
		return mtUnknown, err
	}

	if mt != mtOctetStream {
		return mt, nil
	}

	// The style may not match the content type if it was not specified, so
	// check the headers for the style.
	style := destinationHeader.WhichStyle(r.Header)
	if style == "" {
		style = messageTypeHeader.WhichStyle(r.Header)
		if style == styleXXmidt {
			// Prefer the older format for backward compatibility
			style = styleXWebpa
		}
	}

	return toMediaType(mt.String(), style)
}

// examineRequest parses Accept and picks best + returns parameters
func examineRequest(r *http.Request) (mediaType, error) {
	header := r.Header.Get("Accept")
	if header == "" {
		// No Accept header, return in the form the request was made.
		return examineContentType(r)
	}

	// Parse client Accept header
	parts := strings.Split(header, ",")
	clientAccepted := make([]acceptType, 0, len(parts))

	for _, part := range parts {
		mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(part))
		if err != nil {
			return "", err
		}

		q := 1.0
		if qstr, ok := params["q"]; ok {
			if qf, err := strconv.ParseFloat(qstr, 64); err == nil {
				q = qf
				delete(params, "q") // Remove q so only real params are left
			}
		}

		clientAccepted = append(clientAccepted, acceptType{
			Value:  mediaType,
			Params: params,
			Q:      q,
		})
	}

	// Sort by descending q
	sort.SliceStable(clientAccepted, func(i, j int) bool {
		return clientAccepted[i].Q > clientAccepted[j].Q
	})

	hasWildcardAll := false

	for _, ct := range clientAccepted {
		if ct.Value == "*/*" || ct.Value == "application/*" {
			hasWildcardAll = true
			continue
		}

		// Exact match
		style := strings.ToLower(ct.Params["style"])
		mt, _ := toMediaType(ct.Value, style)
		if mt != mtUnknown {
			return mt, nil
		}
	}

	// Wildcard */* fallback
	if hasWildcardAll {
		// Prefer fast, scalable results when it's up to the server
		return mtMsgpackL, nil
	}

	return mtUnknown, errors.New("no acceptable content type found")
}
