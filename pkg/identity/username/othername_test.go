// Copyright 2022 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package username

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"strings"
	"testing"
)

func TestMarshalAndUnmarshalSANS(t *testing.T) {
	otherName := "foo!example.com"
	critical := true

	ext, err := MarshalSANS(otherName, critical)
	if err != nil {
		t.Fatalf("unexpected error for MarshalSANs: %v", err)
	}
	if ext.Critical != critical {
		t.Fatalf("expected extension to be critical")
	}
	if !ext.Id.Equal(asn1.ObjectIdentifier{2, 5, 29, 17}) {
		t.Fatalf("expected extension's OID to be SANs OID")
	}
	// https://lapo.it/asn1js/#MCGgHwYKKwYBBAGDvzABB6ARDA9mb28hZXhhbXBsZS5jb20
	// 30 - Constructed sequence
	// 21 - length of sequence
	// A0 - Context-specific (class 2) (bits 8,7) with Constructed bit (bit 6) and 0 tag
	// 1F - length of context-specific field (OID)
	// 06 - OID tag
	// 0A - length of OID
	// 2B 06 01 04 01 83 BF 30 01 07 - OID
	// A0 - Context-specific (class 2) with Constructed bit and 0 tag
	//      (needed for EXPLICIT encoding, which wraps field in outer encoding)
	// 11 - length of context-specific field (string)
	// 0C - UTF8String tag
	// 0F - length of string
	// 66 6F 6F 21 65 78 61 6D 70 6C 65 2E 63 6F 6D - string
	if hex.EncodeToString(ext.Value) != "3021a01f060a2b0601040183bf300107a0110c0f666f6f216578616d706c652e636f6d" {
		t.Fatalf("unexpected ASN.1 encoding")
	}

	on, err := UnmarshalSANS([]pkix.Extension{*ext})
	if err != nil {
		t.Fatalf("unexpected error for UnmarshalSANs: %v", err)
	}
	if on != otherName {
		t.Fatalf("unexpected OtherName, expected %s, got %s", otherName, on)
	}
}

func TestUnmarshalSANsFailures(t *testing.T) {
	var err error

	// failure: no SANs extension
	ext := &pkix.Extension{
		Id:       asn1.ObjectIdentifier{},
		Critical: true,
		Value:    []byte{},
	}
	_, err = UnmarshalSANS([]pkix.Extension{*ext})
	if err == nil || !strings.Contains(err.Error(), "no OtherName found") {
		t.Fatalf("expected error finding no OtherName, got %v", err)
	}

	// failure: bad sequence
	ext = &pkix.Extension{
		Id:       asn1.ObjectIdentifier{2, 5, 29, 17},
		Critical: true,
		Value:    []byte{},
	}
	_, err = UnmarshalSANS([]pkix.Extension{*ext})
	if err == nil || !strings.Contains(err.Error(), "sequence truncated") {
		t.Fatalf("expected error with invalid ASN.1, got %v", err)
	}

	// failure: extra data after valid sequence
	b, _ := hex.DecodeString("3021a01f060a2b0601040183bf300107a0110c0f666f6f216578616d706c652e636f6d" + "30")
	ext = &pkix.Extension{
		Id:       asn1.ObjectIdentifier{2, 5, 29, 17},
		Critical: true,
		Value:    b,
	}
	_, err = UnmarshalSANS([]pkix.Extension{*ext})
	if err == nil || !strings.Contains(err.Error(), "trailing data after X.509 extension") {
		t.Fatalf("expected error with extra data, got %v", err)
	}

	// failure: non-universal class (Change last two bits: 30 = 00110000 => 10110000 -> B0)
	b, _ = hex.DecodeString("B021a01f060a2b0601040183bf300107a0110c0f666f6f216578616d706c652e636f6d")
	ext = &pkix.Extension{
		Id:       asn1.ObjectIdentifier{2, 5, 29, 17},
		Critical: true,
		Value:    b,
	}
	_, err = UnmarshalSANS([]pkix.Extension{*ext})
	if err == nil || !strings.Contains(err.Error(), "bad SAN sequence") {
		t.Fatalf("expected error with non-universal class, got %v", err)
	}

	// failure: not compound sequence (Change 6th bit: 30 = 00110000 => 00010000 -> 10)
	b, _ = hex.DecodeString("1021a01f060a2b0601040183bf300107a0110c0f666f6f216578616d706c652e636f6d")
	ext = &pkix.Extension{
		Id:       asn1.ObjectIdentifier{2, 5, 29, 17},
		Critical: true,
		Value:    b,
	}
	_, err = UnmarshalSANS([]pkix.Extension{*ext})
	if err == nil || !strings.Contains(err.Error(), "bad SAN sequence") {
		t.Fatalf("expected error with non-compound sequence, got %v", err)
	}

	// failure: non-sequence tag (Change lower 5 bits: 30 = 00110000 => 00000010 -> 02)
	b, _ = hex.DecodeString("0221a01f060a2b0601040183bf300107a0110c0f666f6f216578616d706c652e636f6d")
	ext = &pkix.Extension{
		Id:       asn1.ObjectIdentifier{2, 5, 29, 17},
		Critical: true,
		Value:    b,
	}
	_, err = UnmarshalSANS([]pkix.Extension{*ext})
	if err == nil || !strings.Contains(err.Error(), "bad SAN sequence") {
		t.Fatalf("expected error with non-sequence tag, got %v", err)
	}

	// failure: no GeneralName with tag=0 (Change lower 5 bits of first sequence field: 3021a01f -> 3021a11f)
	b, _ = hex.DecodeString("3021a11f060a2b0601040183bf300108a0110c0f666f6f216578616d706c652e636f6d")
	ext = &pkix.Extension{
		Id:       asn1.ObjectIdentifier{2, 5, 29, 17},
		Critical: true,
		Value:    b,
	}
	_, err = UnmarshalSANS([]pkix.Extension{*ext})
	if err == nil || !strings.Contains(err.Error(), "no OtherName found") {
		t.Fatalf("expected error with no GeneralName, got %v", err)
	}

	// failure: invalid OtherName (Change tag of UTF8String field to 1: a0110c0f -> a1110c0f)
	b, _ = hex.DecodeString("3021a01f060a2b0601040183bf300108a1110c0f666f6f216578616d706c652e636f6d")
	ext = &pkix.Extension{
		Id:       asn1.ObjectIdentifier{2, 5, 29, 17},
		Critical: true,
		Value:    b,
	}
	_, err = UnmarshalSANS([]pkix.Extension{*ext})
	if err == nil || !strings.Contains(err.Error(), "could not parse requested OtherName SAN") {
		t.Fatalf("expected error with invalid OtherName, got %v", err)
	}

	// failure: OtherName has wrong OID (2b0601040183bf300107 -> 2b0601040183bf300108)
	b, _ = hex.DecodeString("3021a01f060a2b0601040183bf300108a0110c0f666f6f216578616d706c652e636f6d")
	ext = &pkix.Extension{
		Id:       asn1.ObjectIdentifier{2, 5, 29, 17},
		Critical: true,
		Value:    b,
	}
	_, err = UnmarshalSANS([]pkix.Extension{*ext})
	if err == nil || !strings.Contains(err.Error(), "unexpected OID for OtherName") {
		t.Fatalf("expected error with wrong OID, got %v", err)
	}

	// failure: multiple OtherName fields (Increase sequence size from 0x21 -> 0x42, duplicate OtherName)
	b, _ = hex.DecodeString("3042a01f060a2b0601040183bf300107a0110c0f666f6f216578616d706c652e636f6da01f060a2b0601040183bf300107a0110c0f666f6f216578616d706c652e636f6d")
	ext = &pkix.Extension{
		Id:       asn1.ObjectIdentifier{2, 5, 29, 17},
		Critical: true,
		Value:    b,
	}
	_, err = UnmarshalSANS([]pkix.Extension{*ext})
	if err == nil || !strings.Contains(err.Error(), "expected only one OtherName") {
		t.Fatalf("expected error with multiple OtherName fields, got %v", err)
	}
}
