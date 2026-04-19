// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/service/wickr"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// withExtraOptions installs the Wickr JSON-shim BeforeDeserialization
// interceptor on every Wickr client constructed by this service package. See
// [wickrNetworkIDInterceptor] for the motivating bug (aws/aws-sdk-go-v2#3391).
func (p *servicePackage) withExtraOptions(_ context.Context, _ map[string]any) []func(*wickr.Options) {
	return []func(*wickr.Options){
		func(o *wickr.Options) {
			o.Interceptors.AddBeforeDeserialization(wickrNetworkIDInterceptor{})
		},
	}
}

// wickrNumericStringRE matches `"<key>":<digits>` where <key> is one of the
// Wickr response fields that the public API Reference declares as String but
// the live service sends as a JSON number. The regex is intentionally narrow:
// it requires the key to be quoted and immediately followed by `":` (JSON
// object-entry shape) so it does NOT match numeric values inside other
// strings or nested structures that legitimately use numbers. For example,
// `"migrationState":2` is NOT matched because `migrationState` is NOT in
// the key list (it is documented as Integer and correctly returned as one).
//
// Fields currently covered: `networkId`. Add others here ONLY when backed by
// an observed wire response from a live Wickr operation — do not guess.
var wickrNumericStringRE = regexache.MustCompile(`"(networkId)":(\d+)`)

// wickrNetworkIDInterceptor is a BeforeDeserialization interceptor that
// rewrites bare numeric `networkId` values in the HTTP response body to
// quoted strings before the SDK's generated JSON decoder runs. This is a
// targeted workaround for aws/aws-sdk-go-v2#3391: the live AWS Wickr admin
// service violates its own public API Reference by serializing `networkId`
// (documented as `Type: String, Length: Fixed 8, Pattern: [0-9]{8}`) as a
// JSON number (for example `"networkId":53383724` instead of
// `"networkId":"53383724"`). awscli tolerates the mismatch because Python's
// JSON decoder plus botocore's shape-validation layer coerces `int` → `str`
// silently; the strongly-typed Go decoder rejects every response. This
// interceptor brings the wire bytes back into conformance with the
// documented contract so downstream deserialization succeeds.
//
// Remove this interceptor (and the enclosing withExtraOptions hook) once
// the upstream service or SDK fix lands.
type wickrNetworkIDInterceptor struct{}

// BeforeDeserialization implements smithyhttp.InterceptBeforeDeserialization.
// It consumes the current response body, rewrites any bare `"networkId":<N>`
// occurrences to `"networkId":"<N>"`, and replaces the body with the fixed
// bytes so the SDK's generated deserializer sees a schema-conforming
// response.
func (wickrNetworkIDInterceptor) BeforeDeserialization(_ context.Context, in *smithyhttp.InterceptorContext) error {
	if in == nil || in.Response == nil || in.Response.Body == nil {
		return nil
	}

	body, err := io.ReadAll(in.Response.Body)
	_ = in.Response.Body.Close()
	if err != nil {
		return fmt.Errorf("reading Wickr response body for networkId shim: %w", err)
	}

	fixed := wickrNumericStringRE.ReplaceAll(body, []byte(`"$1":"$2"`))
	in.Response.Body = io.NopCloser(bytes.NewReader(fixed))

	return nil
}
