// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/service/wickr"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// withExtraOptions installs the Wickr response-body shim on every Wickr
// client constructed by this service package. The shim rewrites wire-format
// mismatches in the HTTP response body before the SDK's generated
// deserializer runs.
func (p *servicePackage) withExtraOptions(_ context.Context, _ map[string]any) []func(*wickr.Options) {
	return []func(*wickr.Options){
		func(o *wickr.Options) {
			o.APIOptions = append(o.APIOptions, addResponseShim)
		},
	}
}

// addResponseShim adds the response body shim middleware to the
// Deserialize step. It is positioned AFTER the raw response handler
// (AddRawResponse) but BEFORE the OperationDeserializer and the
// ResponseErrorWrapper. This ensures the body is rewritten before any
// deserialization (success or error) occurs.
func addResponseShim(stack *middleware.Stack) error {
	return stack.Deserialize.Insert(&responseShim{}, "OperationDeserializer", middleware.After)
}

// numericStringRE matches `"<key>":<digits>` where <key> is one of the
// Wickr response fields that the public API Reference declares as String but
// the live service sends as a JSON number.
//
// Fields currently covered: `networkId`, `botId`, `groupId`.
var numericStringRE = regexache.MustCompile(`"(networkId|botId|groupId)":(\d+)`)

// messageArrayRE matches `"Message":[` or `"message":[` where the value
// is a JSON array. The Wickr API sometimes returns responses where the
// message field is a JSON array of objects instead of a single string.
// The SDK error types declare `Message *string`, so the Go JSON decoder
// fails with "cannot unmarshal array into Go struct field .Message of type
// string". Case-insensitive match because the API uses lowercase "message"
// while the SDK expects "Message".
//
// Observed on: RegisterOidcConfig (2026-04-23, us-east-1).
var messageArrayRE = regexache.MustCompile(`"[Mm]essage"\s*:\s*\[`)

// responseShim is a Deserialize-step middleware that rewrites the raw
// HTTP response body to fix wire-format mismatches between the live Wickr
// service and the SDK's generated types.
//
//  1. Bare numeric IDs: `"networkId":53383724` → `"networkId":"53383724"`.
//     Workaround for aws/aws-sdk-go-v2#3391.
//
//  2. Array-valued Message: `"Message":["err1","err2"]` → `"Message":"err1; err2"`.
//     The SDK error types declare `Message *string`; the live service
//     sometimes returns an array of strings.
//
// Positioned AFTER OperationDeserializer in the Deserialize step. When
// OperationDeserializer calls next.HandleDeserialize, it reaches this
// middleware. We call next (which triggers the HTTP call), fix the
// response body, and return. OperationDeserializer then reads the fixed
// body.
//
// Remove once the upstream service or SDK fixes land.
type responseShim struct{}

func (*responseShim) ID() string { return "WickrResponseShim" }

func (*responseShim) HandleDeserialize(
	ctx context.Context,
	in middleware.DeserializeInput,
	next middleware.DeserializeHandler,
) (middleware.DeserializeOutput, middleware.Metadata, error) {
	out, md, err := next.HandleDeserialize(ctx, in)

	if rawResp, ok := out.RawResponse.(*smithyhttp.Response); ok && rawResp != nil && rawResp.Body != nil {
		body, readErr := io.ReadAll(rawResp.Body)
		if readErr == nil {
			_ = rawResp.Body.Close()
			fixed := fixResponseBody(body)
			rawResp.Body = io.NopCloser(bytes.NewReader(fixed))
			rawResp.ContentLength = int64(len(fixed))
		}
	}

	return out, md, err
}

// fixResponseBody applies all wire-format fixes to a response body.
func fixResponseBody(body []byte) []byte {
	fixed := numericStringRE.ReplaceAll(body, []byte(`"$1":"$2"`))
	fixed = fixMessageArray(fixed)
	return fixed
}

// fixMessageArray rewrites `"Message":[...]` or `"message":[...]` to a
// single string value. The array may contain strings or objects with
// "field" and "reason" keys. Objects are formatted as "field: reason".
func fixMessageArray(body []byte) []byte {
	if !messageArrayRE.Match(body) {
		return body
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}

	// Check both "Message" and "message" keys.
	var msgRaw json.RawMessage
	var msgKey string
	if v, ok := raw["Message"]; ok {
		msgRaw = v
		msgKey = "Message"
	} else if v, ok := raw[names.AttrMessage]; ok {
		msgRaw = v
		msgKey = names.AttrMessage
	} else {
		return body
	}

	// Try to unmarshal as []string first.
	var msgs []string
	if err := json.Unmarshal(msgRaw, &msgs); err == nil {
		joined := strings.Join(msgs, "; ")
		quotedBytes, err := json.Marshal(joined)
		if err != nil {
			return body
		}
		raw[msgKey] = quotedBytes

		result, err := json.Marshal(raw)
		if err != nil {
			return body
		}
		return result
	}

	// Try to unmarshal as []map[string]string (objects with field/reason).
	var objMsgs []map[string]string
	if err := json.Unmarshal(msgRaw, &objMsgs); err == nil {
		var parts []string
		for _, obj := range objMsgs {
			if reason, ok := obj["reason"]; ok {
				if field, ok := obj[names.AttrField]; ok {
					parts = append(parts, field+": "+reason)
				} else {
					parts = append(parts, reason)
				}
			} else {
				// Fallback: marshal the object back to string.
				b, _ := json.Marshal(obj)
				parts = append(parts, string(b))
			}
		}
		joined := strings.Join(parts, "; ")
		quotedBytes, err := json.Marshal(joined)
		if err != nil {
			return body
		}
		raw[msgKey] = quotedBytes

		result, err := json.Marshal(raw)
		if err != nil {
			return body
		}
		return result
	}

	return body
}
