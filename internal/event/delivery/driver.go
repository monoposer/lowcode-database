package delivery

import (
	"context"
	"net/url"
)

// Driver pushes a message to a parsed delivery URL.
type Driver interface {
	Push(ctx context.Context, u *url.URL, msg Message) error
}
