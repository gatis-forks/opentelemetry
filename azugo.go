// Copyright 2024 Azugo
// SPDX-License-Identifier: Apache-2.0

package opentelemetry

import (
	"context"

	"azugo.io/azugo"
	"azugo.io/core"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log/global"
)

// Use OpenTelemetry for tracing in Azugo application.
func Use(app *azugo.App, config *Configuration, opts ...Option) (core.Tasker, error) {
	shutdownFns := make([]func(context.Context) error, 0, 2)

	if config == nil {
		config = &Configuration{
			TraceLogging: true,
		}
	}

	// If tracing is disabled, return a no-op setup.
	if config.IsDisabled() {
		return &noop{}, nil
	}

	traceProvider, err := newTraceProvider(app, config)
	if err != nil {
		return nil, err
	}

	shutdownFns = append(shutdownFns, traceProvider.Shutdown)

	if config.TraceLogging {
		logProvider, err := newLogProvider(app, config)
		if err != nil {
			return nil, err
		}

		shutdownFns = append(shutdownFns, logProvider.Shutdown)

		global.SetLoggerProvider(logProvider)

		setupOTelLogging(app)
	}

	// Set the global OTEL providers
	otel.SetTextMapPropagator(newPropagator())
	otel.SetTracerProvider(traceProvider)

	app.RouterOptions().PanicHandler = panicHandler

	app.Use(middleware(config, opts...))

	app.Instrumentation(instr(opts...))

	return &setup{
		app:         app,
		config:      config,
		shutdownFns: shutdownFns,
	}, nil
}

func FromContext(ctx context.Context) context.Context {
	c := azugo.RequestContext(ctx)
	if c == nil {
		return ctx
	}

	val := c.UserValue(otelParentSpanContext)
	if val == nil {
		return ctx
	}

	sc, ok := val.(context.Context)
	if !ok {
		return ctx
	}

	return sc
}

type noop struct{}

func (noop) Name() string {
	return "Open Telemetry"
}

func (noop) Start(context.Context) error {
	return nil
}

func (noop) Stop() {}
