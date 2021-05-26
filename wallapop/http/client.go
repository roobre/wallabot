package http

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/google/go-querystring/query"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

const defaultKey = "Tm93IHRoYXQgeW91J3ZlIGZvdW5kIHRoaXMsIGFyZSB5b3UgcmVhZHkgdG8gam9pbiB1cz8gam9ic0B3YWxsYXBvcC5jb20=="
const fingerprintDelimiter = "|"

const signatureHeader = "X-Signature"
const timestampHeader = "Timestamp"

const baseURLv3 = "https://api.wallapop.com/api/v3"

func New() *Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(fmt.Errorf("creating cookie jar: %w", err))
	}

	return &Client{
		HttpClient: &http.Client{
			Transport:     http.DefaultTransport,
			CheckRedirect: nil,
			Jar:           jar,
			Timeout:       5 * time.Second,
		},
		Key: defaultKey,
	}
}

type Client struct {
	HttpClient *http.Client
	Key        string
}

func (c *Client) Request(endpoint string, method string, params interface{}) (*http.Response, error) {
	reqArgs, err := query.Values(params)
	if err != nil {
		return nil, fmt.Errorf("marshalling url params: %w", err)
	}

	u, err := url.Parse(baseURLv3 + endpoint)
	if err != nil {
		return nil, fmt.Errorf("bulding url: %w", err)
	}

	urlArgs := u.Query()
	for k, v := range reqArgs {
		urlArgs.Set(k, v[0])
	}
	u.RawQuery = urlArgs.Encode()

	log.Printf("Requesting %v...", u.String())
	req := &http.Request{
		Method: method,
		URL:    u,
		Header: http.Header{},
	}
	c.addStandardHeaders(req)

	err = c.sign(req)
	if err != nil {
		return nil, fmt.Errorf("signing request: %w", err)
	}

	return c.HttpClient.Do(req)
}

func (c *Client) addStandardHeaders(req *http.Request) {
	for _, h := range []string{
		"Accept\n\tapplication/json, text/plain, */*",
		"Cache-Control\n\tno-cache",
		"DNT\n\t1",
		"Origin\n\thttps://es.wallapop.com",
		"Pragma\n\tno-cache",
		"User-Agent\n\tMozilla/5.0 (X11; Linux x86_64; rv:89.0) Gecko/20100101 Firefox/89.0",
	} {
		hdr := strings.Split(h, "\n\t")
		req.Header.Add(hdr[0], hdr[1])
	}
	req.Header.Set(timestampHeader, fmt.Sprint(time.Now().UTC()))
}

// sign signs a request using Wallapop's dubious scheme.
// Ported from signature.js.
func (c *Client) sign(r *http.Request) error {
	// Join method, path and current time with separator, adding a trailing separator
	fingerprint := strings.Join([]string{
		r.Method,
		r.URL.Path,
		r.Header.Get(timestampHeader),
	}, fingerprintDelimiter) + fingerprintDelimiter

	// Compute HMAC of the fingerprint
	hm := hmac.New(sha256.New, []byte(c.Key))
	hm.Sum([]byte(fingerprint))

	// Encode fingerprint in b64
	signatureBuf := &bytes.Buffer{}
	encoder := base64.NewEncoder(base64.StdEncoding, signatureBuf)
	_, err := encoder.Write(hm.Sum(nil))
	if err != nil {
		return fmt.Errorf("b64 encoding hmac: %w", err)
	}

	// Set header
	r.Header.Set(signatureHeader, signatureBuf.String())
	return nil
}
