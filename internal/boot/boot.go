// Package boot is the composition root: it wires config into the runtime graph.
package boot

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"altalune.id/othra/apperror"
	"altalune.id/othra/config"
	"altalune.id/othra/httpserver"
	"altalune.id/othra/internal/controlplane"
	"altalune.id/othra/internal/health"
	"altalune.id/othra/internal/kernel"
	"altalune.id/othra/internal/oauth/hosted"
	"altalune.id/othra/internal/oauth/oidc"
	"altalune.id/othra/internal/web/layout"
	"altalune.id/othra/scheduler"
	"altalune.id/othra/worker"
)

// Boot builds the runtime graph from cfg: telemetry, the DB pool, migrations, and the services.
func Boot(ctx context.Context, cfg config.Config, log *slog.Logger, opts ...Option) (_ *App, err error) {
	o := newOptions()
	for _, opt := range opts {
		opt(o)
	}
	if o.logger != nil {
		log = o.logger
	}

	tp, mp, shutdown, err := setupTelemetry(ctx, telemetryConfig(cfg.Telemetry), log)
	if err != nil {
		return nil, err
	}

	pool, err := openDB(ctx, cfg, log)
	if err != nil {
		_ = shutdown(ctx)
		return nil, err
	}

	if cfg.DB.AutoMigrate {
		if err := migrate(ctx, cfg.Postgres); err != nil {
			_ = pool.Close()
			_ = shutdown(ctx)
			return nil, err
		}
	}

	if err := guardRLS(ctx, pool, cfg, log); err != nil {
		_ = pool.Close()
		_ = shutdown(ctx)
		return nil, err
	}

	k := kernel.Kernel{
		Log:    log,
		Pool:   pool,
		Tracer: tp.Tracer(kernel.Scope),
		Meter:  mp.Meter(kernel.Scope),
		Reporter: apperror.NewReporter(log, cfg.Env.IsProduction(),
			apperror.WithIncidentCounter(newIncidentCounter(mp)),
			apperror.WithContextMeta(requestMeta),
		),
	}
	k.AddCloser(pool)

	nc, nh, err := buildNATS(ctx, &k, cfg, log)
	if err != nil {
		_ = pool.Close()
		_ = shutdown(ctx)
		return nil, err
	}
	if nc != nil {
		defer func() {
			if err != nil {
				nc.Close()
			}
		}()
	}

	qclient, err := buildQueueFn(ctx, k, cfg, nc, log)
	if err != nil {
		_ = pool.Close()
		_ = shutdown(ctx)
		return nil, err
	}
	k.Queue = qclient

	rdb, rh, err := buildRedis(ctx, &k, cfg, tp, log)
	if err != nil {
		_ = pool.Close()
		_ = shutdown(ctx)
		return nil, err
	}
	if rdb != nil {
		defer func() {
			if err != nil {
				_ = rdb.Close()
			}
		}()
	}

	s3c, sh, err := buildS3(ctx, &k, cfg, log)
	if err != nil {
		_ = pool.Close()
		_ = shutdown(ctx)
		return nil, err
	}
	if s3c != nil {
		defer func() {
			if err != nil {
				_ = s3c.Close()
			}
		}()
	}

	if k.Storage, err = buildStorageDriver(cfg, s3c); err != nil {
		_ = pool.Close()
		_ = shutdown(ctx)
		return nil, err
	}

	svcs := buildServices(k, cfg)
	if svcs.Mailer, err = buildMailer(cfg); err != nil {
		_ = pool.Close()
		_ = shutdown(ctx)
		return nil, err
	}
	lims, err := buildLimiters(cfg, rdb, log)
	if err != nil {
		_ = pool.Close()
		_ = shutdown(ctx)
		return nil, err
	}
	svcs.AuthLimiter = lims.auth
	svcs.Clients.WithFetchLimits(layout.Limits{Limiter: lims.auth, TrustedProxyHops: cfg.HTTP.TrustedProxyHops})
	if svcs.Validator, err = controlplane.NewValidator(); err != nil {
		_ = pool.Close()
		_ = shutdown(ctx)
		return nil, err
	}
	if err := buildAuthSecrets(cfg, svcs, log); err != nil {
		_ = pool.Close()
		_ = shutdown(ctx)
		return nil, err
	}
	if err := firstBoot(ctx, cfg, svcs, log, logsInstallURL(o)); err != nil {
		_ = pool.Close()
		_ = shutdown(ctx)
		return nil, err
	}
	if svcs.InstallRequired, err = installRequired(ctx, svcs); err != nil {
		_ = pool.Close()
		_ = shutdown(ctx)
		return nil, err
	}
	warnSampleLegal(cfg, log, logsInstallURL(o))
	warnUpstreamSwitches(cfg.Upstreams, log)
	dbHealth := buildDBHealth(k, cfg, log)
	apiHandler := buildAPIHandler(svcs, k.Reporter.Unexpected, cfg.API, cfg.HTTP.BasePath, log)
	crossSite := slices.Concat(hosted.CrossSitePOSTs(cfg.HTTP.BasePath), oidc.CrossSitePOSTs(cfg.HTTP.BasePath))
	crossOrigin, err := buildCrossOrigin(cfg.HTTP.BaseURL, crossSite)
	if err != nil {
		_ = pool.Close()
		_ = shutdown(ctx)
		return nil, err
	}
	consoleHandler := buildConsoleHandler(cfg.HTTP.BasePath)
	_, portalAssets := buildPortal(cfg.HTTP.BasePath)
	issuerPublic, hostedPages := buildIssuerRoutes(cfg, svcs, log, k.Reporter.Unexpected)
	mcpServer := buildMCPServer(cfg.MCP, svcs, log, mcpPublicHost(cfg.HTTP)...)

	sup := worker.New(log)
	sup.Register(httpserver.New(
		httpConfig(cfg.HTTP),
		buildHTTPHandler(httpSurfaces{
			log:        log,
			unexpected: k.Reporter.Unexpected,
			probes:     health.NewWeb(health.NewService(readinessChecks(dbHealth, rh, nh, sh)...)),
			basePath:   cfg.HTTP.BasePath,
			api:        apiHandler,
			console:    consoleHandler,
			portal:     portalAssets,
			mcp:        buildMCPHandler(mcpServer, svcs, log),
			routes:     buildRoutes(cfg, svcs, log, k.Reporter.Unexpected),
			public:     issuerPublic,
			hosted:     hostedPages,
			images:     hostedImageBases(cfg.Storage),

			installRequired: svcs.InstallRequired,
			session:         buildSessionMiddleware(svcs, k.Reporter.Unexpected),
			consoleGate:     buildConsoleGate(cfg, svcs),

			requireAcceptance: cfg.Compliance.RequireAcceptance && svcs.Caps.LocalLogin,
			termsUpdated:      svcs.Legal.TermsUpdatedAt(),
			crossOrigin:       crossOrigin,
		}),
		log,
	))
	sup.Register(dbHealth)
	if rh != nil {
		sup.Register(rh)
	}
	if nh != nil {
		sup.Register(nh)
	}
	if sh != nil {
		sup.Register(sh)
	}
	sup.Register(buildOutboxWorker(svcs, log))
	sup.Register(buildMailWorker(svcs, log))
	sup.Register(svcs.KeyUsage)
	if svcs.TokenUsage != nil {
		sup.Register(svcs.TokenUsage)
	}
	if lims.janitor != nil {
		sup.Register(lims.janitor)
	}

	var runner *scheduler.Runner
	if off := schedulerOff(o, cfg); off == "" {
		r, schedErr := buildScheduler(k, cfg, svcs, log)
		if schedErr != nil {
			_ = pool.Close()
			_ = shutdown(ctx)
			return nil, schedErr
		}
		runner = r
		sup.Register(runner)
	} else if log != nil {
		log.Info("boot: scheduler disabled by " + off)
	}
	if o.schedulerOnly && runner == nil {
		_ = pool.Close()
		_ = shutdown(ctx)
		return nil, fmt.Errorf("boot: scheduler-only requires the scheduler, but it is disabled by %s", schedulerOff(o, cfg))
	}

	var consumer worker.Worker
	if off := consumerOff(o, cfg); off == "" {
		c, cErr := k.Queue.Consumer(ctx, queueHandlers(svcs))
		if cErr != nil {
			_ = pool.Close()
			_ = shutdown(ctx)
			return nil, fmt.Errorf("boot: queue consumer: %w", cErr)
		}
		consumer = c
		sup.Register(consumer)
	} else if log != nil {
		log.Info("boot: queue consumer disabled by " + off)
	}
	if o.consumerOnly && consumer == nil {
		_ = pool.Close()
		_ = shutdown(ctx)
		return nil, fmt.Errorf("boot: consumer-only requires the queue consumer, but it is disabled by %s", consumerOff(o, cfg))
	}
	if cfg.Queue.Enabled {
		listener, lErr := k.Queue.Listener(ctx, queueListeners(svcs))
		if lErr != nil {
			_ = pool.Close()
			_ = shutdown(ctx)
			return nil, fmt.Errorf("boot: queue listener: %w", lErr)
		}
		sup.Register(listener)
	}

	return &App{
		Kernel:            k,
		Services:          svcs,
		Supervisor:        sup,
		MCP:               mcpServer,
		shutdownTelemetry: shutdown,
	}, nil
}
