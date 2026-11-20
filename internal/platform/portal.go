package platform

import (
	"context"

	"afa/internal/diagnostic"
	"afa/internal/portal"
)

const PortalGuidance = "Tarayıcı açıldığında giriş sayfasının ağ sağlayıcısına ait olduğunu doğrulayın."

// OpenPortal asks the operating system to open the fixed connectivity probe.
// A successful handoff does not indicate that a login took place.
func OpenPortal(ctx context.Context) error {
	return openPortal(ctx, openURL)
}

func openPortal(ctx context.Context, launch func(context.Context, string) error) error {
	if err := ctx.Err(); err != nil {
		return diagnostic.Error{Code: diagnostic.CodeOf(err, diagnostic.LaunchFailure)}
	}
	if err := launch(ctx, portal.ProbeURL); err != nil {
		return diagnostic.Error{Code: diagnostic.CodeOf(err, diagnostic.LaunchFailure)}
	}
	return nil
}
