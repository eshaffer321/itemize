package sync

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/eshaffer321/monarch-go/v2/pkg/monarch"
	"github.com/stretchr/testify/assert"
)

func TestIsRetryableMonarchError(t *testing.T) {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name string
		ctx  context.Context
		err  error
		want bool
	}{
		{
			name: "caller canceled",
			ctx:  canceledCtx,
			err:  monarch.ErrServerError,
			want: false,
		},
		{
			name: "canceled error",
			ctx:  context.Background(),
			err:  context.Canceled,
			want: false,
		},
		{
			name: "Monarch server error",
			ctx:  context.Background(),
			err:  monarch.ErrServerError,
			want: true,
		},
		{
			name: "network timeout",
			ctx:  context.Background(),
			err: &url.Error{
				Op:  "Post",
				URL: "https://api.monarch.com/graphql",
				Err: context.DeadlineExceeded,
			},
			want: true,
		},
		{
			name: "permanent error",
			ctx:  context.Background(),
			err:  errors.New("invalid category"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isRetryableMonarchError(tt.ctx, tt.err))
		})
	}
}
