package gdrive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
)

type OAuthManager struct {
	clientID     string
	clientSecret string
	tokenPath    string
	config       *oauth2.Config
}

// NewOAuthManager initializes the OAuthManager config
func NewOAuthManager(clientID, clientSecret, tokenPath string) *OAuthManager {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{drive.DriveScope}, // Full drive scope
		RedirectURL:  "http://localhost:8080/callback",
	}
	return &OAuthManager{
		clientID:     clientID,
		clientSecret: clientSecret,
		tokenPath:    tokenPath,
		config:       config,
	}
}

// GetClient returns an authenticated HTTP client, initiating the login flow if needed
func (m *OAuthManager) GetClient(ctx context.Context) (*http.Client, error) {
	token, err := m.tokenFromFile()
	if err != nil {
		// Token doesn't exist or is invalid, trigger web flow
		token, err = m.runWebAuthFlow(ctx)
		if err != nil {
			return nil, err
		}
		if err := m.saveToken(token); err != nil {
			return nil, err
		}
	}

	// Use TokenSource to automatically handle refresh tokens
	tokenSource := m.config.TokenSource(ctx, token)
	return oauth2.NewClient(ctx, tokenSource), nil
}

func (m *OAuthManager) tokenFromFile() (*oauth2.Token, error) {
	f, err := os.Open(m.tokenPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func (m *OAuthManager) saveToken(token *oauth2.Token) error {
	f, err := os.OpenFile(m.tokenPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

func (m *OAuthManager) runWebAuthFlow(ctx context.Context) (*oauth2.Token, error) {
	codeChan := make(chan string)
	errChan := make(chan error)

	server := &http.Server{Addr: ":8080"}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "Authentication failed: code missing")
			errChan <- fmt.Errorf("authorization code missing in callback")
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Authentication successful! You can close this window now.")
		codeChan <- code
	})

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Auth URL with AccessTypeOffline and ApprovalForce to request a refresh token
	authURL := m.config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Por favor, abre este enlace en tu navegador para autorizar la aplicación:\n%s\n", authURL)
	}

	select {
	case code := <-codeChan:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)

		return m.config.Exchange(ctx, code)
	case err := <-errChan:
		_ = server.Shutdown(context.Background())
		return nil, err
	case <-ctx.Done():
		_ = server.Shutdown(context.Background())
		return nil, ctx.Err()
	}
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
}
