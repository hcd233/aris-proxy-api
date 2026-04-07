package oauth2_config

import (
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

type testCase struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Platform     string `json:"platform"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURL  string `json:"redirect_url"`
}

func loadCases(t *testing.T) []testCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures: %v", err)
	}
	var cases []testCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures: %v", err)
	}
	return cases
}

func findCase(t *testing.T, cases []testCase, name string) testCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("case %q not found", name)
	return testCase{}
}

func buildOAuth2Config(tc testCase) *oauth2.Config {
	var endpoint oauth2.Endpoint
	switch tc.Platform {
	case "github":
		endpoint = github.Endpoint
	case "google":
		endpoint = google.Endpoint
	default:
		endpoint = github.Endpoint
	}
	return &oauth2.Config{
		ClientID:     tc.ClientID,
		ClientSecret: tc.ClientSecret,
		RedirectURL:  tc.RedirectURL,
		Endpoint:     endpoint,
		Scopes:       []string{"user:email"},
	}
}

func validateConfig(cfg *oauth2.Config) error {
	if cfg.ClientID == "" {
		return &validationError{field: "ClientID"}
	}
	if cfg.ClientSecret == "" {
		return &validationError{field: "ClientSecret"}
	}
	return nil
}

type validationError struct {
	field string
}

func (e *validationError) Error() string {
	return e.field + " is empty"
}

func TestValidateConfig_EmptyClientID(t *testing.T) {
	allCases := loadCases(t)

	names := []string{"empty_client_id_github", "empty_client_id_google"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			tc := findCase(t, allCases, name)
			cfg := buildOAuth2Config(tc)
			err := validateConfig(cfg)
			if err == nil {
				t.Errorf("expected validation error for empty ClientID, got nil")
			}
			t.Logf("validation error: %v", err)
		})
	}
}

func TestValidateConfig_EmptyClientSecret(t *testing.T) {
	allCases := loadCases(t)

	names := []string{"empty_client_secret_github", "empty_client_secret_google"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			tc := findCase(t, allCases, name)
			cfg := buildOAuth2Config(tc)
			err := validateConfig(cfg)
			if err == nil {
				t.Errorf("expected validation error for empty ClientSecret, got nil")
			}
			t.Logf("validation error: %v", err)
		})
	}
}

func TestValidateConfig_BothEmpty(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "both_empty_github")
	cfg := buildOAuth2Config(tc)
	err := validateConfig(cfg)
	if err == nil {
		t.Errorf("expected validation error for both empty, got nil")
	}
	t.Logf("validation error: %v", err)
}

func TestValidateConfig_ValidConfig(t *testing.T) {
	allCases := loadCases(t)

	names := []string{"valid_github_config", "valid_google_config"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			tc := findCase(t, allCases, name)
			cfg := buildOAuth2Config(tc)
			err := validateConfig(cfg)
			if err != nil {
				t.Errorf("expected no error for valid config, got %v", err)
			}
		})
	}
}

func TestAuthURL_ContainsClientID(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "valid_config_url_has_client_id")
	cfg := buildOAuth2Config(tc)

	authURL := cfg.AuthCodeURL("test-state", oauth2.AccessTypeOffline)
	t.Logf("auth URL: %s", authURL)

	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("failed to parse auth URL: %v", err)
	}

	clientID := parsed.Query().Get("client_id")
	if clientID != tc.ClientID {
		t.Errorf("auth URL client_id = %q, want %q", clientID, tc.ClientID)
	}
}

func TestAuthURL_EmptyClientIDProducesBrokenURL(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "empty_client_id_github")
	cfg := buildOAuth2Config(tc)

	authURL := cfg.AuthCodeURL("test-state", oauth2.AccessTypeOffline)
	t.Logf("auth URL with empty client_id: %s", authURL)

	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("failed to parse auth URL: %v", err)
	}

	clientID := parsed.Query().Get("client_id")
	if clientID != "" {
		t.Errorf("expected empty client_id in URL, got %q", clientID)
	}

	if !strings.Contains(authURL, "client_id=&") && !strings.HasSuffix(authURL, "client_id=") {
		t.Errorf("expected broken client_id= in URL, got %s", authURL)
	}
}
