package gitrepod

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"

	"golang.org/x/crypto/ssh"
)

func genSSHPrivateKey(keyPath string) (ssh.Signer, error) {
	randPath := "/dev/urandom"

	r, err := os.Open(randPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %s", randPath, err)
	}
	priv, err := rsa.GenerateKey(r, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate rsa key: %s", err)
	}
	der := x509.MarshalPKCS1PrivateKey(priv)
	b := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
	if err := ioutil.WriteFile(keyPath, b, 0600); err != nil {
		return nil, fmt.Errorf("failed to write private key: %s", err)
	}
	return ssh.NewSignerFromKey(priv)
}

func loadSSHPrivateKey(keyPath string) (ssh.Signer, error) {
	privateBytes, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s:%s", keyPath, err)
	}
	return ssh.ParsePrivateKey(privateBytes)
}

// LoadSSHAuthorizedKey returns ssh pubkey from path
func LoadSSHAuthorizedKey(keyPath string) (ssh.PublicKey, error) {
	publicBytes, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s:%s", keyPath, err)
	}
	pub, _, _, _, err := ssh.ParseAuthorizedKey(publicBytes)
	return pub, err
}

// SameSSHPubkeys reports whether two publickey the same key.
func SameSSHPubkeys(a, b ssh.PublicKey) bool {
	return bytes.Compare(a.Marshal(), b.Marshal()) == 0
}
