package login

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"

	"github.com/stripe/stripe-cli/pkg/profile"
	"github.com/stripe/stripe-cli/pkg/stripeauth"
	"github.com/stripe/stripe-cli/pkg/validators"
)

var execCommand = exec.Command

const stripeCLIAuthURL = "https://dashboard.stripe.com/stripecli/auth"

// Links provides the URLs for the CLI to continue the login flow
type Links struct {
	BrowserURL       string `json:"browser_url"`
	PollURL          string `json:"poll_url"`
	VerificationCode string `json:"verification_code"`
}

//TODO
/*
4. Observability and associated alerting? Business metrics (how many users use this flow)?
5. Rate limiting for each operation?
6. Audit trail for key generation
7. Move configuration changes to profile package
*/

// Login function is used to obtain credentials via stripe dashboard.
func Login(url string, profile profile.Profile) error {

	links, err := getLinks(url, profile.DeviceName)
	if err != nil {
		return err
	}

	fmt.Printf("Opening login link %s in your browser.\nVerification code is %s\n", links.BrowserURL, links.VerificationCode)

	urlErr := openBrowser(links.BrowserURL)
	if urlErr != nil {
		return urlErr
	}

	//Call poll function
	apiKey, err := PollForKey(links.PollURL, 0, 0)
	if err != nil {
		return err
	}

	validateErr := validators.APIKey(apiKey)
	if validateErr != nil {
		return validateErr
	}

	configErr := profile.ConfigureProfile(apiKey)
	if configErr != nil {
		return configErr
	}

	return nil
}

func openBrowser(url string) error {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = execCommand("xdg-open", url).Start()
	case "windows":
		err = execCommand("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = execCommand("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	if err != nil {
		return err
	}

	return nil
}

func getLinks(authURL string, deviceName string) (*Links, error) {
	client := stripeauth.NewHTTPClient("")

	if authURL == "" {
		authURL = stripeCLIAuthURL
	}

	data := url.Values{}
	data.Set("device_name", deviceName)

	res, err := client.PostForm(authURL, data)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected http status code: %d %s", res.StatusCode, string(bodyBytes))
	}

	var links Links
	err = json.Unmarshal(bodyBytes, &links)
	if err != nil {
		return nil, err
	}

	return &links, nil
}
