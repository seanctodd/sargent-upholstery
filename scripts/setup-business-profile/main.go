// Command setup-business-profile is a one-time tool to authorize the Google
// Business Profile API
// and retrieve your location name + refresh token for use in CI.
//
// Usage:
//
//	go run ./scripts/setup-business-profile /path/to/client_secret.json
//
// Prerequisites (do this once in Google Cloud Console):
//  1. Use the SAME project that was granted Business Profile API access
//     (the access request and the OAuth client must live in one project, or
//     every call gets quota 0).
//  2. Enable, in that project: "My Business Account Management API",
//     "My Business Business Information API", and "Google My Business API"
//     (the legacy v4 API that serves reviews).
//  3. Create an OAuth 2.0 credential → type "Desktop app".
//  4. Download the JSON file Google provides.
//
// Note on hosts: account/location discovery uses the newer v1 APIs
// (mybusinessaccountmanagement / mybusinessbusinessinformation); only the
// reviews endpoint still lives on the legacy mybusiness.googleapis.com/v4 host.
//
// After running this script, add these GitHub Actions secrets:
//
//	GOOGLE_CLIENT_ID
//	GOOGLE_CLIENT_SECRET
//	GOOGLE_REFRESH_TOKEN  (printed by this script)
//	GOOGLE_LOCATION_NAME  (printed by this script, e.g. accounts/123/locations/456)

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Google's downloaded credentials JSON format
type credentialsFile struct {
	Installed *oauthCredentials `json:"installed"`
	Web       *oauthCredentials `json:"web"`
}

type oauthCredentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// httpClient mirrors the timeout used by the fetch scripts; the bare
// http.DefaultClient has none and can hang indefinitely.
var httpClient = &http.Client{Timeout: 30 * time.Second}

const (
	tokenURL    = "https://oauth2.googleapis.com/token"
	accountsURL = "https://mybusinessaccountmanagement.googleapis.com/v1/accounts"
	scope       = "https://www.googleapis.com/auth/business.manage"
	// 127.0.0.1 rather than "localhost": the listener binds to 127.0.0.1 only,
	// and "localhost" can resolve to ::1 first, which would be refused. Google
	// accepts either form for Desktop-app OAuth clients without registration.
	redirectURI = "http://127.0.0.1:8080"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run ./scripts/setup-business-profile /path/to/client_secret.json")
		os.Exit(1)
	}

	credData, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading credentials file: %v\n", err)
		os.Exit(1)
	}
	var creds credentialsFile
	if err := json.Unmarshal(credData, &creds); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing credentials file: %v\n", err)
		os.Exit(1)
	}
	c := creds.Installed
	if c == nil {
		c = creds.Web
	}
	if c == nil || c.ClientID == "" || c.ClientSecret == "" {
		fmt.Fprintln(os.Stderr, "Could not find client_id/client_secret in credentials file")
		os.Exit(1)
	}
	clientID := c.ClientID
	clientSecret := c.ClientSecret
	fmt.Printf("Loaded credentials for client: %s\n", clientID)

	// Build authorization URL
	authURL := "https://accounts.google.com/o/oauth2/v2/auth?" + url.Values{
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {scope},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
	}.Encode()

	// Start a local server to capture the redirect. Bind to loopback only: the
	// OAuth authorization code arrives in this request's query string, and
	// listening on every interface would expose it to the local network.
	codeCh := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			fmt.Fprintln(w, "No authorization code received.")
			return
		}
		fmt.Fprintln(w, "Authorization successful! You can close this tab.")
		codeCh <- code
	})
	srv := &http.Server{Handler: mux}

	listener, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting local server: %v\n", err)
		os.Exit(1)
	}
	go srv.Serve(listener)

	// Open browser
	fmt.Println("Opening browser for Google authorization...")
	fmt.Println("If the browser doesn't open, visit this URL manually:")
	fmt.Println(authURL)
	openBrowser(authURL)

	// Wait for code
	fmt.Println("Waiting for authorization...")
	code := <-codeCh
	srv.Shutdown(context.Background())

	// Exchange code for tokens
	resp, err := httpClient.PostForm(tokenURL, url.Values{
		"code":          {code},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Token exchange error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var tokens struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Error        string `json:"error"`
	}
	if err := json.Unmarshal(body, &tokens); err != nil || tokens.Error != "" {
		fmt.Fprintf(os.Stderr, "Token parse error: %v\nBody: %s\n", err, body)
		os.Exit(1)
	}
	if tokens.RefreshToken == "" {
		fmt.Fprintln(os.Stderr, "No refresh token received. Try revoking access at https://myaccount.google.com/permissions and re-running.")
		os.Exit(1)
	}

	// Fetch accounts
	locationName, err := findLocation(tokens.AccessToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching location: %v\n", err)
		fmt.Println("\nCould not auto-detect location. List your locations manually and find the one matching your business.")
		fmt.Println("Refresh token (save this):", tokens.RefreshToken)
		os.Exit(1)
	}

	fmt.Println("\n========== GitHub Actions Secrets ==========")
	fmt.Println("Go to: github.com/seanctodd/sargent-upholstery/settings/secrets/actions")
	fmt.Println("Add these repository secrets:")
	fmt.Println()
	fmt.Printf("  GOOGLE_CLIENT_ID     = %s\n", clientID)
	fmt.Printf("  GOOGLE_CLIENT_SECRET = %s\n", clientSecret)
	fmt.Printf("  GOOGLE_REFRESH_TOKEN = %s\n", tokens.RefreshToken)
	fmt.Printf("  GOOGLE_LOCATION_NAME = %s\n", locationName)
	fmt.Println("============================================")
}

func findLocation(accessToken string) (string, error) {
	// List accounts
	body, err := apiGet(accountsURL, accessToken)
	if err != nil {
		return "", err
	}
	var accountsResp struct {
		Accounts []struct {
			Name        string `json:"name"`
			AccountName string `json:"accountName"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(body, &accountsResp); err != nil {
		return "", fmt.Errorf("parse accounts: %w", err)
	}
	if len(accountsResp.Accounts) == 0 {
		return "", fmt.Errorf("no accounts found")
	}

	// Try each account's locations
	for _, account := range accountsResp.Accounts {
		locURL := fmt.Sprintf("https://mybusinessbusinessinformation.googleapis.com/v1/%s/locations?readMask=name,title,storeCode,metadata&pageSize=100", account.Name)
		locBody, err := apiGet(locURL, accessToken)
		if err != nil {
			continue
		}
		var locsResp struct {
			Locations []struct {
				Name     string `json:"name"`
				Title    string `json:"title"`
				Metadata struct {
					MapsURI string `json:"mapsUri"`
					PlaceID string `json:"placeId"`
				} `json:"metadata"`
			} `json:"locations"`
		}
		if err := json.Unmarshal(locBody, &locsResp); err != nil {
			continue
		}
		// v1 returns location names as "locations/{id}"; the reviews v4 endpoint
		// needs the full "accounts/{id}/locations/{id}" parent, so join them.
		for _, loc := range locsResp.Locations {
			fullName := account.Name + "/" + loc.Name
			if strings.Contains(strings.ToLower(loc.Title), "sargent") ||
				strings.Contains(loc.Metadata.MapsURI, "ChIJvQmmCSS35YgR") ||
				loc.Metadata.PlaceID == "ChIJvQmmCSS35YgR-3H9ajzGCHk" {
				fmt.Printf("Found location: %s (%s)\n", loc.Title, fullName)
				return fullName, nil
			}
		}
		// If only one location, use it
		if len(locsResp.Locations) == 1 {
			loc := locsResp.Locations[0]
			fullName := account.Name + "/" + loc.Name
			fmt.Printf("Using only location found: %s (%s)\n", loc.Title, fullName)
			return fullName, nil
		}
		// Print all locations for manual selection
		if len(locsResp.Locations) > 0 {
			fmt.Printf("Locations in account %s (%s):\n", account.AccountName, account.Name)
			for _, loc := range locsResp.Locations {
				fmt.Printf("  %s  →  %s\n", loc.Title, account.Name+"/"+loc.Name)
			}
		}
	}
	return "", fmt.Errorf("could not find Sargent Upholstery location — see locations printed above and set GOOGLE_LOCATION_NAME manually")
}

func apiGet(url, accessToken string) ([]byte, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}
	return body, nil
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}
	exec.Command(cmd, args...).Start()
}
