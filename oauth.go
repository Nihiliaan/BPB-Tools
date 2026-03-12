package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"

	cf "github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/accounts"
	"github.com/cloudflare/cloudflare-go/v4/option"
	"golang.org/x/oauth2"
)

//go:embed static/index.html
var indexHTML []byte

var (
	cfClient      *cf.Client
	cfAccount     *accounts.Account
	state         string
	codeVerifier  string
	obtainedToken = make(chan *oauth2.Token)
	config        = &oauth2.Config{
		ClientID:     "54d11594-84e4-41aa-b438-e81b8fa78ee7",
		ClientSecret: "",
		RedirectURL:  "http://localhost:8976/oauth/callback",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://dash.cloudflare.com/oauth2/auth",
			TokenURL: "https://dash.cloudflare.com/oauth2/token",
		},
		Scopes: []string{
			"account:read", "user:read", "workers:write", "workers_kv:write",
			"workers_routes:write", "workers_scripts:write", "workers_tail:read",
			"d1:write", "pages:write", "pages:read", "zone:read", "ssl_certs:write",
			"ai:write", "queues:write", "pipelines:write", "secrets_store:write",
		},
	}
)

func NewClient(token *oauth2.Token) *cf.Client {
	return cf.NewClient(option.WithAPIToken(token.AccessToken))
}

func getAccount(ctx context.Context) (*accounts.Account, error) {
	res, err := cfClient.Accounts.List(ctx, accounts.AccountListParams{})
	if err != nil {
		return nil, fmt.Errorf("error listing accounts - %v", err)
	}

	return &res.Result[0], nil
}

func generateAuthURL() string {
	state = generateState()
	codeVerifier = generateCodeVerifier()
	codeChallenge := generateCodeChallenge(codeVerifier)

	return config.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

func generateState() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	buff := make([]byte, 16)

	for i := range buff {
		buff[i] = charset[rand.Int64()%int64(len(charset))]
	}

	return base64.URLEncoding.EncodeToString(buff)
}

func generateCodeVerifier() string {
	b := make([]byte, 32)

	for i := range b {
		b[i] = byte(rand.IntN(256))
	}

	return base64.RawURLEncoding.EncodeToString(b)
}

func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func login() {
	url := generateAuthURL()
	fmt.Printf("\n%s 登录 %s...\n", title, fmtStr("Cloudflare", ORANGE, true))

	if err := openURL(url); err != nil {
		failMessage("登录失败。")
		log.Fatalln(err)
	}
}

func callback(w http.ResponseWriter, r *http.Request) {
	param := r.URL.Query().Get("state")
	if param != state {
		failMessage("OAuth 状态验证失败。")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		failMessage("未收到授权码。")
		return
	}

	token, err := config.Exchange(
		context.Background(),
		code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	)

	if err != nil {
		failMessage("令牌交换失败。")
		log.Fatalln(err)
	}

	obtainedToken <- token
	successMessage("Cloudflare 登录成功！")

	w.Header().Set("Content-Type", "text/html")
	w.Write(indexHTML)
}
