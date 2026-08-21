package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/credential"
	tradingmigration "github.com/LyricTian/gin-admin/v10/internal/mods/trading/migration"
	"github.com/LyricTian/gin-admin/v10/internal/wirex"
	"github.com/urfave/cli/v2"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func ImportNOFXCmd() *cli.Command {
	return &cli.Command{
		Name:  "import-nofx",
		Usage: "Dry-run or import legacy NOFX data into the configured gin-admin database",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "workdir", Aliases: []string{"d"}, Value: "configs", Usage: "Configuration working directory"},
			&cli.StringFlag{Name: "config", Aliases: []string{"c"}, Value: "dev", Usage: "Target runtime configuration files or directory"},
			&cli.StringFlag{Name: "source-type", Value: "mysql", Usage: "Legacy database type: mysql, postgres, or sqlite3"},
			&cli.StringFlag{Name: "source-dsn", Usage: "Legacy NOFX database DSN", Required: true},
			&cli.StringFlag{Name: "source-name", Value: "nofx", Usage: "Stable legacy source identifier used for idempotency"},
			&cli.StringFlag{Name: "default-owner", Usage: "gin-admin user ID used when no explicit legacy user mapping exists"},
			&cli.StringFlag{Name: "owner-map-file", Usage: "JSON file mapping legacy user IDs to gin-admin user IDs"},
			&cli.BoolFlag{Name: "apply", Usage: "Persist imported data; omitted means dry-run only"},
		},
		Action: runNOFXImport,
	}
}

func runNOFXImport(c *cli.Context) error {
	ownerMap, err := loadOwnerMap(c.String("owner-map-file"))
	if err != nil {
		return err
	}
	if strings.TrimSpace(c.String("default-owner")) == "" && len(ownerMap) == 0 {
		return errors.New("default-owner or owner-map-file is required")
	}

	if err := config.Load(c.String("workdir"), strings.Split(c.String("config"), ",")...); err != nil {
		return err
	}
	config.C.General.WorkDir = c.String("workdir")
	config.C.PreLoad()

	target, closeTarget, err := wirex.InitDB(c.Context)
	if err != nil {
		return fmt.Errorf("open target database: %w", err)
	}
	defer closeTarget()

	source, err := openLegacyDB(c.String("source-type"), c.String("source-dsn"))
	if err != nil {
		return fmt.Errorf("open legacy database: %w", err)
	}
	if sqlDB, dbErr := source.DB(); dbErr == nil {
		defer sqlDB.Close()
	}

	credentials := credential.NewServiceFromEnv()
	if c.Bool("apply") && !credentials.Ready() {
		return credential.ErrKeyNotConfigured
	}
	if !c.Bool("apply") && !credentials.Ready() {
		// Dry-run still exercises encryption paths without persisting this ephemeral key.
		credentials = credential.NewService(make([]byte, 32), nil)
	}

	importer := &tradingmigration.Importer{Source: source, Target: target, Credentials: credentials}
	report, importErr := importer.Run(tradingmigration.Options{
		SourceName:         c.String("source-name"),
		DefaultOwnerUserID: c.String("default-owner"),
		OwnerMap:           ownerMap,
		DryRun:             !c.Bool("apply"),
	})
	if report != nil {
		encoded, marshalErr := json.MarshalIndent(report, "", "  ")
		if marshalErr != nil {
			return marshalErr
		}
		fmt.Fprintln(c.App.Writer, string(encoded))
	}
	return importErr
}

func openLegacyDB(dbType, dsn string) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch strings.ToLower(strings.TrimSpace(dbType)) {
	case "mysql":
		dialector = mysql.Open(dsn)
	case "postgres", "postgresql":
		dialector = postgres.Open(dsn)
	case "sqlite", "sqlite3":
		dialector = sqlite.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported legacy database type %q", dbType)
	}
	return gorm.Open(dialector, &gorm.Config{Logger: logger.Discard})
}

func loadOwnerMap(path string) (map[string]string, error) {
	result := make(map[string]string)
	if strings.TrimSpace(path) == "" {
		return result, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read owner map: %w", err)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse owner map: %w", err)
	}
	return result, nil
}
