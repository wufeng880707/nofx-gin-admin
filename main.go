package main

import (
	"os"

	"github.com/LyricTian/gin-admin/v10/cmd"
	"github.com/urfave/cli/v2"
)

// Usage: go build -ldflags "-X main.VERSION=x.x.x"
var VERSION = "v10.1.0"

// @title XTrading API
// @version v10.1.0
// @description XTrading management API for exchange accounts, AI models, strategies, traders, runtime observations and RBAC operations.
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
// @schemes http https
// @basePath /
func main() {
	app := cli.NewApp()
	app.Name = "xtrading"
	app.Version = VERSION
	app.Usage = "XTrading management server and trading runtime operations."
	app.Commands = []*cli.Command{
		cmd.StartCmd(),
		cmd.StopCmd(),
		cmd.ImportNOFXCmd(),
		cmd.PasswordHashCmd(),
		cmd.VersionCmd(VERSION),
	}
	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}
}
