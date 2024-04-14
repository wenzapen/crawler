package auth

import (
	"context"
	"errors"
	"strings"

	"go-micro.dev/v4"
	"go-micro.dev/v4/auth"
	"go-micro.dev/v4/metadata"
	"go-micro.dev/v4/server"
)

func NewAuthWrapper(service micro.Service) server.HandlerWrapper {
	return func(h server.HandlerFunc) server.HandlerFunc {
		return func(ctx context.Context, req server.Request, rsp interface{}) error {
			mb, b := metadata.FromContext(ctx)
			if !b {
				return errors.New("no metadata found")
			}
			authHeader, ok := mb["Authorization"]
			if !ok || !strings.HasPrefix(authHeader, auth.BearerScheme) {
				return errors.New("no auth token provided")
			}
			token := strings.TrimPrefix(authHeader, auth.BearerScheme)
			a := service.Options().Auth
			_, err := a.Inspect(token)
			if err != nil {
				return errors.New("auth token invalid")
			}

			return h(ctx, req, rsp)
		}
	}
}
