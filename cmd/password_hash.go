package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/LyricTian/gin-admin/v10/pkg/crypto/hash"
	"github.com/urfave/cli/v2"
)

const (
	minimumPasswordLength = 12
	maximumPasswordLength = 72
)

// PasswordHashCmd generates a bcrypt hash without accepting passwords in argv.
func PasswordHashCmd() *cli.Command {
	return &cli.Command{
		Name:  "password-hash",
		Usage: "Read a password from stdin and print its bcrypt hash",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "password-stdin", Usage: "Read the password from standard input"},
		},
		Action: func(c *cli.Context) error {
			if !c.Bool("password-stdin") {
				return fmt.Errorf("--password-stdin is required; passwords in command arguments are not supported")
			}
			password, err := io.ReadAll(io.LimitReader(os.Stdin, maximumPasswordLength+2))
			if err != nil {
				return fmt.Errorf("read password: %w", err)
			}
			value := strings.TrimRight(string(password), "\r\n")
			if len(value) < minimumPasswordLength {
				return fmt.Errorf("password must contain at least %d bytes", minimumPasswordLength)
			}
			if len(value) > maximumPasswordLength {
				return fmt.Errorf("password must contain at most %d bytes", maximumPasswordLength)
			}
			hashed, err := hash.GeneratePassword(value)
			if err != nil {
				return fmt.Errorf("hash password: %w", err)
			}
			fmt.Fprintln(c.App.Writer, hashed)
			return nil
		},
	}
}
