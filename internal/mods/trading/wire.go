package trading

import (
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/api"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/assistant"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/biz"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/credential"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/dal"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/llm"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/provider"
	tradingruntime "github.com/LyricTian/gin-admin/v10/internal/mods/trading/runtime"
	"github.com/google/wire"
)

// Set contains the NOFX trading module dependencies.
var Set = wire.NewSet(
	wire.Struct(new(Trading), "*"),
	wire.Struct(new(dal.Catalog), "*"),
	wire.Struct(new(dal.Observation), "*"),
	wire.Struct(new(dal.Analytics), "*"),
	wire.Struct(new(dal.Execution), "*"),
	wire.Struct(new(dal.Competition), "*"),
	wire.Struct(new(dal.SpotStrategy), "*"),
	wire.Struct(new(biz.Catalog), "*"),
	wire.Struct(new(biz.Observation), "*"),
	wire.Struct(new(biz.Analytics), "*"),
	biz.NewExecution,
	wire.Struct(new(biz.Competition), "*"),
	biz.NewSpotStrategy,
	wire.Struct(new(api.Catalog), "*"),
	wire.Struct(new(api.Observation), "*"),
	wire.Struct(new(api.Analytics), "*"),
	wire.Struct(new(api.Execution), "*"),
	wire.Struct(new(api.Competition), "*"),
	wire.Struct(new(api.Assistant), "*"),
	wire.Struct(new(api.SpotStrategy), "*"),
	credential.NewServiceFromEnv,
	llm.NewService,
	provider.NewFactory,
	assistant.NewService,
	tradingruntime.NewRegistry,
)
