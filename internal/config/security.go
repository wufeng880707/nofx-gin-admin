package config

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const minimumSigningKeyLength = 32

var knownWeakSigningKeys = map[string]struct{}{
	"XnEsT0S@":             {},
	"CG24SDVP8OHPK395GB5G": {},
	"ChangeThisJwtSigningKeyWithAtLeast32Chars": {},
	"jwt_signing_key_1234567890123456":          {},
}

// ValidateSecurity rejects credentials and authentication settings that are
// acceptable for local development but unsafe for a production deployment.
func (c *Config) ValidateSecurity() error {
	if !c.General.Security.Strict {
		return nil
	}
	if c.General.Debug {
		return fmt.Errorf("strict security mode requires General.Debug=false")
	}
	if !c.General.DisablePrintConfig {
		return fmt.Errorf("strict security mode requires General.DisablePrintConfig=true")
	}
	if c.Middleware.Auth.Disable {
		return fmt.Errorf("strict security mode does not allow authentication to be disabled")
	}
	if err := validateSigningKey("Middleware.Auth.SigningKey", c.Middleware.Auth.SigningKey); err != nil {
		return err
	}
	if oldKey := c.Middleware.Auth.OldSigningKey; oldKey != "" {
		if err := validateSigningKey("Middleware.Auth.OldSigningKey", oldKey); err != nil {
			return err
		}
		if oldKey == c.Middleware.Auth.SigningKey {
			return fmt.Errorf("Middleware.Auth.OldSigningKey must differ from SigningKey")
		}
	}
	if strings.TrimSpace(c.General.Root.Username) == "" {
		return fmt.Errorf("General.Root.Username is required in strict security mode")
	}
	if _, err := bcrypt.Cost([]byte(c.General.Root.PasswordHash)); err != nil {
		return fmt.Errorf("General.Root.PasswordHash must be a valid bcrypt hash: %w", err)
	}
	if c.Util.Prometheus.Enable &&
		(c.Util.Prometheus.BasicUsername == "admin" || c.Util.Prometheus.BasicPassword == "admin") {
		return fmt.Errorf("default Prometheus credentials are not allowed in strict security mode")
	}
	return nil
}

func validateSigningKey(name, key string) error {
	if len(key) < minimumSigningKeyLength {
		return fmt.Errorf("%s must contain at least %d bytes", name, minimumSigningKeyLength)
	}
	if _, weak := knownWeakSigningKeys[key]; weak {
		return fmt.Errorf("%s uses a known example or default value", name)
	}
	return nil
}
