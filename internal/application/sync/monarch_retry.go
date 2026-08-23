package sync

import (
	"context"
	"errors"
	"net"

	"github.com/eshaffer321/monarch-go/v2/pkg/monarch"
)

func isRetryableMonarchError(ctx context.Context, err error) bool {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return false
	}
	if monarch.IsRetryable(err) {
		return true
	}

	var networkErr net.Error
	return errors.As(err, &networkErr) && networkErr.Timeout()
}
