// Command devauth mints a development JWT and serves the matching JWKS so the
// birb API can verify it — a local stand-in for Supabase Auth. Run it, copy
// the two VITE_ lines into web/.env.local, and point the API at the JWKS:
//
//	SUPABASE_JWKS_URL=http://localhost:9999/.well-known/jwks.json
//	SUPABASE_JWT_ISSUER=http://localhost:9999
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

const (
	keyFile = "tools/devauth/dev-key.pem"
	// users.auth_id is a Postgres uuid column, so the subject must be a UUID.
	devUID = "00000000-0000-4000-8000-000000000000"
	issuer = "http://localhost:9999"
	addr   = ":9999"
)

func main() {
	priv := loadOrCreateKey()

	key, err := jwk.FromRaw(priv)
	fatal(err)
	fatal(key.Set(jwk.KeyIDKey, "dev-key-1"))
	fatal(key.Set(jwk.AlgorithmKey, jwa.RS256))

	pub, err := key.PublicKey()
	fatal(err)
	set := jwk.NewSet()
	fatal(set.AddKey(pub))
	jwks, err := json.Marshal(set)
	fatal(err)

	tok, err := jwt.NewBuilder().
		Issuer(issuer).
		Subject(devUID).
		Audience([]string{"authenticated"}).
		Claim("email", "dev@birb.local").
		Claim("name", "Dev User").
		IssuedAt(time.Now()).
		Expiration(time.Now().Add(30 * 24 * time.Hour)).
		Build()
	fatal(err)

	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256, key))
	fatal(err)

	fmt.Printf("VITE_DEV_TOKEN=%s\n", signed)
	fmt.Printf("VITE_DEV_AUTH_UID=%s\n\n", devUID)
	fmt.Printf("serving JWKS on http://localhost%s/.well-known/jwks.json\n", addr)

	http.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwks)
	})
	log.Fatal(http.ListenAndServe(addr, nil))
}

func loadOrCreateKey() *rsa.PrivateKey {
	if raw, err := os.ReadFile(keyFile); err == nil {
		block, _ := pem.Decode(raw)
		if block != nil {
			if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
				return k
			}
		}
	}
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	fatal(err)
	raw := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)})
	fatal(os.WriteFile(keyFile, raw, 0o600))
	return k
}

func fatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
