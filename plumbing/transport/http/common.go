// Package http implements the HTTP transport protocol.
package http

import (
	"fmt"
	"net/http"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
)

type client struct {
	c *http.Client
}

// DefaultClient is the default HTTP client, which uses `http.DefaultClient`.
var DefaultClient = NewClient(nil)

// NewClient creates a new client with a custom net/http client.
// See `InstallProtocol` to install and override default http client.
// Unless a properly initialized client is given, it will fall back into
// `http.DefaultClient`.
//
// Note that for HTTP client cannot distinguist between private repositories and
// unexistent repositories on GitHub. So it returns `ErrAuthorizationRequired`
// for both.
func NewClient(c *http.Client) transport.Transport {
	if c == nil {
		return &client{http.DefaultClient}
	}

	return &client{
		c: c,
	}
}

func (c *client) NewUploadPackSession(ep transport.Endpoint, auth transport.AuthMethod) (
	transport.UploadPackSession, error) {

	return newUploadPackSession(c.c, ep, auth)
}

func (c *client) NewReceivePackSession(ep transport.Endpoint, auth transport.AuthMethod) (
	transport.ReceivePackSession, error) {

	return newReceivePackSession(c.c, ep, auth)
}

type session struct {
	auth     AuthMethod
	client   *http.Client
	endpoint transport.Endpoint
	advRefs  *packp.AdvRefs
}

func (*session) Close() error {
	return nil
}

func (s *session) applyAuthToRequest(req *http.Request) {
	if s.auth == nil {
		return
	}

	s.auth.setAuth(req)
}

// AuthMethod is concrete implementation of common.AuthMethod for HTTP services
type AuthMethod interface {
	transport.AuthMethod
	setAuth(r *http.Request)
}

func basicAuthFromEndpoint(ep transport.Endpoint) *BasicAuth {
	u := ep.User()
	if u == "" {
		return nil
	}

	return NewBasicAuth(u, ep.Password())
}

// BasicAuth represent a HTTP basic auth
type BasicAuth struct {
	username, password string
}

// NewBasicAuth returns a basicAuth base on the given user and password
func NewBasicAuth(username, password string) *BasicAuth {
	return &BasicAuth{username, password}
}

func (a *BasicAuth) setAuth(r *http.Request) {
	if a == nil {
		return
	}

	r.SetBasicAuth(a.username, a.password)
}

// Name is name of the auth
func (a *BasicAuth) Name() string {
	return "http-basic-auth"
}

func (a *BasicAuth) String() string {
	masked := "*******"
	if a.password == "" {
		masked = "<empty>"
	}

	return fmt.Sprintf("%s - %s:%s", a.Name(), a.username, masked)
}

// Err is a dedicated error to return errors based on status code
type Err struct {
	Response *http.Response
}

// NewErr returns a new Err based on a http response
func NewErr(r *http.Response) error {
	if r.StatusCode >= http.StatusOK && r.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	switch r.StatusCode {
	case http.StatusUnauthorized:
		return transport.ErrAuthenticationRequired
	case http.StatusForbidden:
		return transport.ErrAuthorizationFailed
	case http.StatusNotFound:
		return transport.ErrRepositoryNotFound
	}

	return plumbing.NewUnexpectedError(&Err{r})
}

// StatusCode returns the status code of the response
func (e *Err) StatusCode() int {
	return e.Response.StatusCode
}

func (e *Err) Error() string {
	return fmt.Sprintf("unexpected requesting %q status code: %d",
		e.Response.Request.URL, e.Response.StatusCode,
	)
}
