package login

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stripe/stripe-cli/pkg/profile"
)

func TestLogin(t *testing.T) {
	if os.Getenv("OPEN_URL") == "1" {
		os.Exit(0)
		return
	}

	execCommand = func(string, ...string) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=TestLogin")
		cmd.Env = []string{"OPEN_URL=1"}
		return cmd
	}
	defer func() { execCommand = exec.Command }()

	configFile := filepath.Join(os.TempDir(), "stripe", "config.toml")
	p := profile.Profile{
		Color:       "auto",
		ConfigFile:  configFile,
		LogLevel:    "info",
		ProfileName: "tests",
		DeviceName:  "st-testing",
	}

	var pollURL string
	var browserURL string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "auth") {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			expectedLinks := Links{
				BrowserURL:       browserURL,
				PollURL:          pollURL,
				VerificationCode: "dinosaur-pineapple-polkadot",
			}
			json.NewEncoder(w).Encode(expectedLinks)
		}
		if strings.Contains(r.URL.Path, "browser") {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<HTML></HTML>"))

		}
		if strings.Contains(r.URL.Path, "poll") {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			data := []byte(`{"redeemed":  true, "account_id": "acct_123", "testmode_key_secret": "sk_test_1234"}`)
			fmt.Println(string(data))
			w.Write(data)
		}
	}))
	defer ts.Close()

	authURL := fmt.Sprintf("%s%s", ts.URL, "/auth")
	pollURL = fmt.Sprintf("%s%s", ts.URL, "/poll")
	browserURL = fmt.Sprintf("%s%s", ts.URL, "/browser")

	err := Login(authURL, p)
	assert.NoError(t, err)
}

func TestGetLinks(t *testing.T) {
	expectedLinks := Links{
		BrowserURL:       "https://stripe.com/browser",
		PollURL:          "https://stripe.com/poll",
		VerificationCode: "dinosaur-pineapple-polkadot",
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		assert.NoError(t, r.ParseForm())
		assert.Equal(t, "test", r.PostFormValue("device_name"))

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedLinks)
	}))
	defer ts.Close()

	links, err := getLinks(ts.URL, "test")
	assert.NoError(t, err)
	assert.Equal(t, expectedLinks, *links)
}

func TestGetLinksHTTPStatusError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	links, err := getLinks(ts.URL, "test")
	assert.EqualError(t, err, "unexpected http status code: 500 ")
	assert.Empty(t, links)
}

func TestGetLinksRequestError(t *testing.T) {
	// Immediately close the HTTP server so that the request fails.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.Close()

	links, err := getLinks(ts.URL, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connect: connection refused")
	assert.Empty(t, links)
}

func TestGetLinksParseError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")

		badLinks := make(map[string]int)
		badLinks["browser_url"] = 10
		json.NewEncoder(w).Encode(badLinks)
	}))
	defer ts.Close()

	links, err := getLinks(ts.URL, "test")
	assert.EqualError(t, err, "json: cannot unmarshal number into Go struct field Links.browser_url of type string")
	assert.Empty(t, links)
}
