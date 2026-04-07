package image_service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// parseDataURLCase represents a test case for ParseDataURL
type parseDataURLCase struct {
	Name              string `json:"name"`
	DataURL           string `json:"data_url"`
	ExpectedMediaType string `json:"expected_media_type"`
	ExpectedBase64    string `json:"expected_base64"`
	ExpectedError     bool   `json:"expected_error"`
}

// checksumCase represents a test case for ComputeImageChecksum
type checksumCase struct {
	Name        string `json:"name"`
	Base64Data  string `json:"base64_data"`
	Description string `json:"description"`
}

// loadParseDataURLCases loads test cases from fixtures/cases.json
func loadParseDataURLCases(t *testing.T) []parseDataURLCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []parseDataURLCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	return cases
}

// loadChecksumCases loads test cases from fixtures/checksum_cases.json
func loadChecksumCases(t *testing.T) []checksumCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/checksum_cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/checksum_cases.json: %v", err)
	}
	var cases []checksumCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/checksum_cases.json: %v", err)
	}
	return cases
}

// findParseCase finds a test case by name
func findParseCase(t *testing.T, cases []parseDataURLCase, name string) parseDataURLCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("test case %q not found in fixtures", name)
	return parseDataURLCase{}
}

// findChecksumCase finds a checksum test case by name
func findChecksumCase(t *testing.T, cases []checksumCase, name string) checksumCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("test case %q not found in fixtures", name)
	return checksumCase{}
}

func TestParseDataURL_ValidJPEG(t *testing.T) {
	cases := loadParseDataURLCases(t)
	tc := findParseCase(t, cases, "valid_jpeg_data_url")

	mediaType, base64Data, err := util.ParseDataURL(tc.DataURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mediaType != tc.ExpectedMediaType {
		t.Errorf("ParseDataURL() mediaType = %q, want %q", mediaType, tc.ExpectedMediaType)
	}
	if base64Data != tc.ExpectedBase64 {
		t.Errorf("ParseDataURL() base64Data = %q, want %q", base64Data, tc.ExpectedBase64)
	}
}

func TestParseDataURL_ValidPNG(t *testing.T) {
	cases := loadParseDataURLCases(t)
	tc := findParseCase(t, cases, "valid_png_data_url")

	mediaType, base64Data, err := util.ParseDataURL(tc.DataURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mediaType != tc.ExpectedMediaType {
		t.Errorf("ParseDataURL() mediaType = %q, want %q", mediaType, tc.ExpectedMediaType)
	}
	if base64Data != tc.ExpectedBase64 {
		t.Errorf("ParseDataURL() base64Data = %q, want %q", base64Data, tc.ExpectedBase64)
	}
}

func TestParseDataURL_ValidGIF(t *testing.T) {
	cases := loadParseDataURLCases(t)
	tc := findParseCase(t, cases, "valid_gif_data_url")

	mediaType, base64Data, err := util.ParseDataURL(tc.DataURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mediaType != tc.ExpectedMediaType {
		t.Errorf("ParseDataURL() mediaType = %q, want %q", mediaType, tc.ExpectedMediaType)
	}
	if base64Data != tc.ExpectedBase64 {
		t.Errorf("ParseDataURL() base64Data = %q, want %q", base64Data, tc.ExpectedBase64)
	}
}

func TestParseDataURL_ValidWebP(t *testing.T) {
	cases := loadParseDataURLCases(t)
	tc := findParseCase(t, cases, "valid_webp_data_url")

	mediaType, base64Data, err := util.ParseDataURL(tc.DataURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mediaType != tc.ExpectedMediaType {
		t.Errorf("ParseDataURL() mediaType = %q, want %q", mediaType, tc.ExpectedMediaType)
	}
	if base64Data != tc.ExpectedBase64 {
		t.Errorf("ParseDataURL() base64Data = %q, want %q", base64Data, tc.ExpectedBase64)
	}
}

func TestParseDataURL_InvalidNoBase64Separator(t *testing.T) {
	cases := loadParseDataURLCases(t)
	tc := findParseCase(t, cases, "invalid_no_base64_separator")

	_, _, err := util.ParseDataURL(tc.DataURL)
	if (err != nil) != tc.ExpectedError {
		t.Errorf("ParseDataURL() error = %v, wantError %v", err, tc.ExpectedError)
	}
}

func TestParseDataURL_InvalidEmptyMediaType(t *testing.T) {
	cases := loadParseDataURLCases(t)
	tc := findParseCase(t, cases, "invalid_empty_media_type")

	_, _, err := util.ParseDataURL(tc.DataURL)
	if (err != nil) != tc.ExpectedError {
		t.Errorf("ParseDataURL() error = %v, wantError %v", err, tc.ExpectedError)
	}
}

func TestParseDataURL_InvalidEmptyData(t *testing.T) {
	cases := loadParseDataURLCases(t)
	tc := findParseCase(t, cases, "invalid_empty_data")

	_, _, err := util.ParseDataURL(tc.DataURL)
	if (err != nil) != tc.ExpectedError {
		t.Errorf("ParseDataURL() error = %v, wantError %v", err, tc.ExpectedError)
	}
}

func TestParseDataURL_InvalidPlainURL(t *testing.T) {
	cases := loadParseDataURLCases(t)
	tc := findParseCase(t, cases, "invalid_plain_url")

	_, _, err := util.ParseDataURL(tc.DataURL)
	if (err != nil) != tc.ExpectedError {
		t.Errorf("ParseDataURL() error = %v, wantError %v", err, tc.ExpectedError)
	}
}

func TestComputeImageChecksum_Deterministic(t *testing.T) {
	cases := loadChecksumCases(t)
	tc := findChecksumCase(t, cases, "simple_bytes")

	rawBytes, err := base64.StdEncoding.DecodeString(tc.Base64Data)
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}

	checksum1 := util.ComputeImageChecksum(rawBytes)
	checksum2 := util.ComputeImageChecksum(rawBytes)
	if checksum1 != checksum2 {
		t.Errorf("ComputeImageChecksum() not deterministic: %q != %q", checksum1, checksum2)
	}

	expected := sha256.Sum256(rawBytes)
	expectedHex := hex.EncodeToString(expected[:])
	if checksum1 != expectedHex {
		t.Errorf("ComputeImageChecksum() = %q, want %q", checksum1, expectedHex)
	}
}

func TestComputeImageChecksum_EmptyInput(t *testing.T) {
	cases := loadChecksumCases(t)
	tc := findChecksumCase(t, cases, "empty_bytes")

	var rawBytes []byte
	if tc.Base64Data != "" {
		var err error
		rawBytes, err = base64.StdEncoding.DecodeString(tc.Base64Data)
		if err != nil {
			t.Fatalf("failed to decode base64: %v", err)
		}
	}

	checksum := util.ComputeImageChecksum(rawBytes)

	expected := sha256.Sum256(rawBytes)
	expectedHex := hex.EncodeToString(expected[:])
	if checksum != expectedHex {
		t.Errorf("ComputeImageChecksum(nil) = %q, want %q", checksum, expectedHex)
	}
}

func TestComputeImageChecksum_DifferentInputs(t *testing.T) {
	cases := loadChecksumCases(t)

	tc1 := findChecksumCase(t, cases, "simple_bytes")
	tc2 := findChecksumCase(t, cases, "single_byte")

	rawBytes1, err := base64.StdEncoding.DecodeString(tc1.Base64Data)
	if err != nil {
		t.Fatalf("failed to decode base64 for %q: %v", tc1.Name, err)
	}
	rawBytes2, err := base64.StdEncoding.DecodeString(tc2.Base64Data)
	if err != nil {
		t.Fatalf("failed to decode base64 for %q: %v", tc2.Name, err)
	}

	checksum1 := util.ComputeImageChecksum(rawBytes1)
	checksum2 := util.ComputeImageChecksum(rawBytes2)

	if checksum1 == checksum2 {
		t.Errorf("ComputeImageChecksum() same for different inputs: %q", checksum1)
	}
}

func TestImageMediaTypeExtensions(t *testing.T) {
	expected := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/gif":  ".gif",
		"image/webp": ".webp",
	}

	for mediaType, wantExt := range expected {
		t.Run(mediaType, func(t *testing.T) {
			gotExt := util.ImageMediaTypeExtensions[mediaType]
			if gotExt != wantExt {
				t.Errorf("ImageMediaTypeExtensions[%q] = %q, want %q", mediaType, gotExt, wantExt)
			}
		})
	}

	t.Run("unknown_type", func(t *testing.T) {
		gotExt := util.ImageMediaTypeExtensions["image/bmp"]
		if gotExt != "" {
			t.Errorf("ImageMediaTypeExtensions[\"image/bmp\"] = %q, want empty", gotExt)
		}
	})
}
