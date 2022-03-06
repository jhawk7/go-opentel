package opentel

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

var traceProvider *sdktrace.TracerProvider
var meterProvider *controller.Controller

func InitOpentelProviders(environment string, otelExporterUrl string, serviceName string) (err error) {
	//setup Opentelemetry trace and meter providers to be used across application
	tp, tpErr := initTraceProvider(otelExporterUrl, serviceName, environment)
	if tpErr != nil {
		err = tpErr
		return
	}

	otel.SetTracerProvider(tp)
	traceProvider = tp

	mp, mpErr := initMeterProvider(otelExporterUrl, serviceName, environment)
	if mpErr != nil {
		err = mpErr
		return
	}

	global.SetMeterProvider(mp)
	meterProvider = mp
	if startErr := mp.Start(context.Background()); startErr != nil {
		err = fmt.Errorf("error starting meter provider collection; %v", startErr)
		return
	}

	return
}

func GetTraceProvider() trace.TracerProvider {
	return otel.GetTracerProvider()
}

func GetMeterProvider() metric.MeterProvider {
	return global.GetMeterProvider()
}

func ShutdownOpentelProviders() (err error) {
	ctx := context.Background()
	if tpErr := traceProvider.Shutdown(ctx); tpErr != nil {
		err = fmt.Errorf("failed to shut down trace provider; %v", tpErr)
		return
	}

	if mpErr := meterProvider.Stop(ctx); mpErr != nil {
		err = fmt.Errorf("failed to shut down meter provider", mpErr)
		return
	}
	ctx.Done()
	return
}

func initTraceProvider(exporterUrl string, serviceName string, environment string) (tp *sdktrace.TracerProvider, tpErr error) {
	//configure grpc exporter
	log.Infof("exporting opentelemetry data to %s", exporterUrl)
	exporter, expErr := otlptracegrpc.New(context.Background(), otlptracegrpc.WithEndpoint(exporterUrl))
	if expErr != nil {
		tpErr = expErr
		return
	}

	//configure trace provider resource to describe this application
	r := getAppResource(serviceName, environment)

	//register exporter with new trace provider
	tp = sdktrace.NewTracerProvider(
		//register exporter with trace provider using BatchSpanProcessor
		sdktrace.WithBatcher(exporter),
		//configure resource to be used in all traces from trace provider
		sdktrace.WithResource(r),
		//setup sampler to always sample traces
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	return
}

func initMeterProvider(exporterUrl string, serviceName string, environment string) (mp *controller.Controller, mpErr error) {
	exporter, expErr := otlpmetricgrpc.New(context.Background(), otlpmetricgrpc.WithEndpoint(exporterUrl))
	if expErr != nil {
		mpErr = expErr
		return
	}

	r := getAppResource(serviceName, environment)

	mp = controller.New(
		processor.NewFactory(
			simple.NewWithHistogramDistribution(),
			exporter,
		),
		//configure exporter for metrics
		controller.WithExporter(exporter),
		//configure resource for metrics
		controller.WithResource(r),
		controller.WithCollectPeriod(controller.DefaultPeriod),
	)

	return
}

func getAppResource(serviceName string, environment string) *resource.Resource {
	//configures resource to describe this application
	r, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
			attribute.String("environment", environment),
		),
	)
	return r
}
