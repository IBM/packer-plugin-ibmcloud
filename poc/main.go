// POC: IAM token exchange authentication
//
// Performs the iam-authz grant against IBM Cloud IAM and prints the resulting
// scoped access token. Run with:
//
//	export MY_ACCESS_TOKEN="<your existing bearer token>"
//	export DESIRED_IAM_ID="crn:v1:bluemix:public:..."
//	go run ./poc/
//
// Optional:
//
//	export TOKEN_EXCHANGE_URL="https://iam.cloud.ibm.com/identity/token"
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const defaultTokenExchangeURL = "https://iam.cloud.ibm.com/identity/token"

func main() {
	accessToken := os.Getenv("MY_ACCESS_TOKEN")
	if accessToken == "" {
		fmt.Fprintln(os.Stderr, "error: MY_ACCESS_TOKEN env var is required")
		os.Exit(1)
	}

	desiredIAMID := os.Getenv("DESIRED_IAM_ID")
	if desiredIAMID == "" {
		fmt.Fprintln(os.Stderr, "error: DESIRED_IAM_ID env var is required")
		os.Exit(1)
	}

	exchangeURL := os.Getenv("TOKEN_EXCHANGE_URL")
	if exchangeURL == "" {
		exchangeURL = defaultTokenExchangeURL
	}

	fmt.Printf("Exchange URL : %s\n", exchangeURL)
	fmt.Printf("Desired IAM ID: %s\n", desiredIAMID)
	fmt.Println("Fetching scoped access token...")

	token, err := fetchAccessToken(accessToken, desiredIAMID, exchangeURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("\nScoped access token:")
	fmt.Println(token)
}

// fetchAccessToken exchanges an existing IBM Cloud access token for a scoped
// token tied to desiredIAMID by posting the iam-authz grant to exchangeURL.
// This is the function that will be copied into step_create_vpc_service_instance.go.
func fetchAccessToken(accessToken, desiredIAMID, exchangeURL string) (string, error) {
	body := url.Values{}
	body.Set("grant_type", "urn:ibm:params:oauth:grant-type:iam-authz")
	body.Set("access_token", accessToken)
	body.Set("desired_iam_id", desiredIAMID)

	req, err := http.NewRequest(http.MethodPost, exchangeURL, strings.NewReader(body.Encode()))
	if err != nil {
		return "", fmt.Errorf("building token exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("posting to token exchange endpoint %s: %w", exchangeURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading token exchange response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token exchange returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parsing token exchange response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("token exchange response contained no access_token field: %s", string(respBody))
	}

	return result.AccessToken, nil
}
