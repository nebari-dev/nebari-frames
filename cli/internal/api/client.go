package api

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
	"github.com/nebari-dev/nebari-frames/gen/go/frames/v1/framesv1connect"
)

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithToken sets the bearer token used for authenticated requests.
func WithToken(token string) ClientOption {
	return func(c *Client) { c.token = token }
}

// Client wraps the generated FrameServiceClient with convenience methods.
type Client struct {
	svc   framesv1connect.FrameServiceClient
	token string
}

// NewClient constructs a Client pointed at baseURL. If a token is provided via
// WithToken, all requests will carry an Authorization: Bearer header.
func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{}
	for _, opt := range opts {
		opt(c)
	}
	httpClient := http.DefaultClient
	if c.token != "" {
		httpClient = &http.Client{Transport: &tokenRoundTripper{base: http.DefaultTransport, token: c.token}}
	}
	c.svc = framesv1connect.NewFrameServiceClient(httpClient, baseURL)
	return c
}

type tokenRoundTripper struct {
	base  http.RoundTripper
	token string
}

func (t *tokenRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(req)
}

// Publish publishes a new frame version. Returns the frame and the new version.
func (c *Client) Publish(ctx context.Context, content []byte, changelog string) (*framesv1.Frame, *framesv1.FrameVersion, error) {
	resp, err := c.svc.PublishFrame(ctx, connect.NewRequest(&framesv1.PublishFrameRequest{
		Content:   content,
		Changelog: changelog,
	}))
	if err != nil {
		return nil, nil, err
	}
	return resp.Msg.Frame, resp.Msg.Version, nil
}

// List returns all frames visible to the caller and whether the caller can
// create new frames.
func (c *Client) List(ctx context.Context) ([]*framesv1.FrameSummary, bool, error) {
	resp, err := c.svc.ListFrames(ctx, connect.NewRequest(&framesv1.ListFramesRequest{}))
	if err != nil {
		return nil, false, err
	}
	return resp.Msg.Frames, resp.Msg.CanCreate, nil
}

// Get fetches a single frame by org slug, name, and optional version.
func (c *Client) Get(ctx context.Context, orgSlug, name, version string) (*framesv1.GetFrameResponse, error) {
	resp, err := c.svc.GetFrame(ctx, connect.NewRequest(&framesv1.GetFrameRequest{
		OrgSlug: orgSlug,
		Name:    name,
		Version: version,
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

// Resolve returns the inheritance-resolved YAML content for a frame.
func (c *Client) Resolve(ctx context.Context, orgSlug, name, version string) ([]byte, error) {
	resp, err := c.svc.ResolveFrame(ctx, connect.NewRequest(&framesv1.ResolveFrameRequest{
		OrgSlug: orgSlug,
		Name:    name,
		Version: version,
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.ResolvedContent, nil
}

// Me returns the identity of the authenticated caller.
func (c *Client) Me(ctx context.Context) (*framesv1.GetMeResponse, error) {
	resp, err := c.svc.GetMe(ctx, connect.NewRequest(&framesv1.GetMeRequest{}))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}
