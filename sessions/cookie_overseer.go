package sessions

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/pkg/errors"
)

// CookieOverseer oversees cookie operations that are encrypted and verified
// but does store all data client side which means it is a possible attack
// vector. Uses GCM to verify and encrypt data.
type CookieOverseer struct {
	options CookieOptions

	secretKey    [32]byte
	gcmBlockMode cipher.AEAD
}

// NewCookieOverseer creates an overseer from cookie options and a secret key
// for use in encryption. Panic's on any errors that deal with cryptography.
func NewCookieOverseer(opts CookieOptions, secretKey [32]byte) *CookieOverseer {
	block, err := aes.NewCipher(secretKey[:])
	if err != nil {
		panic(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}

	return &CookieOverseer{
		options:      opts,
		secretKey:    secretKey,
		gcmBlockMode: gcm,
	}
}

// MakeSecretKey creates a randomized key securely for use with the AES-GCM
// algorithm the size of the key is 32 bytes in order to use AES-256. It must
// be persisted somewhere in order to re-use it across restarts of the app.
func MakeSecretKey() ([32]byte, error) {
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		return key, err
	}
	return key, nil
}

// Get a value from the cookie overseer
func (c *CookieOverseer) Get(w http.ResponseWriter, r *http.Request) (string, error) {
	// Abuse request to be able to easily parse out cookies
	request := http.Request{
		Header: w.Header(),
	}
	cookies := request.Cookies()
	for _, cookie := range cookies {
		if cookie.Name == c.options.Name {
			return cookie.Value, nil
		}
	}

	cookie, err := r.Cookie(c.options.Name)
	if err != nil {
		return "", err
	}

	return c.decode(cookie.Value)
}

// encode into base64'd aes-gcm
func (c *CookieOverseer) encode(plaintext string) (string, error) {
	nonce := make([]byte, c.gcmBlockMode.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", errors.Wrap(err, "failed to encode session cookie")
	}

	// Append ciphertext to the end of nonce so we have the nonce for decrypt
	ciphertext := c.gcmBlockMode.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decode base64'd aes-gcm
func (c *CookieOverseer) decode(ciphertext string) (string, error) {
	ct, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", nil
	}

	if len(ct) <= c.gcmBlockMode.NonceSize() {
		return "", errors.New("failed to decode in cookie overseer")
	}

	// Nonce comes from the first n bytes (n = NonceSize)
	plaintext, err := c.gcmBlockMode.Open(nil,
		ct[:c.gcmBlockMode.NonceSize()],
		ct[c.gcmBlockMode.NonceSize():],
		nil)

	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// Put a value into the cookie overseer
func (c *CookieOverseer) Put(w http.ResponseWriter, r *http.Request, value string) (*http.Request, error) {
	return nil, errors.New("not impl")
}

// Del a value from the cookie overseer
func (c *CookieOverseer) Del(w http.ResponseWriter, r *http.Request) error {
	return errors.New("not impl")
}
