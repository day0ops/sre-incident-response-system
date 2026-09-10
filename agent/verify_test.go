// verify_test.go
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func startTestJWKS(t *testing.T, priv *rsa.PrivateKey, kid string) *httptest.Server {
	t.Helper()
	n := base64.RawURLEncoding.EncodeToString(priv.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1})
	jwks := map[string]any{
		"keys": []map[string]any{
			{"kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256", "n": n, "e": e},
		},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(jwks)
	}))
}

func signTestToken(t *testing.T, priv *rsa.PrivateKey, kid, iss, aud string, exp time.Time) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": iss, "aud": aud, "exp": exp.Unix(), "sub": "test-user",
	})
	token.Header["kid"] = kid
	signed, err := token.SignedString(priv)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func TestVerifyBearer(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	jwks := startTestJWKS(t, priv, "test-kid")
	defer jwks.Close()

	const issuer = "https://login.microsoftonline.com/test-tenant/v2.0"
	const audience = "agent-gateway"

	cases := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{"valid", signTestToken(t, priv, "test-kid", issuer, audience, time.Now().Add(time.Hour)), false},
		{"expired", signTestToken(t, priv, "test-kid", issuer, audience, time.Now().Add(-time.Hour)), true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/task", nil)
			req.Header.Set("Authorization", "Bearer "+c.token)
			_, err := verifyBearer(req, jwks.URL, issuer, audience)
			if c.wantErr && err == nil {
				t.Fatalf("expected error, got none")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
