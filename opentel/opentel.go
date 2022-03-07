package opentel

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
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
var ctx context.Context

func InitOpentelProviders() (err error) {
	//setup Opentelemetry trace and meter providers to be used across application
	context := context.Background()
	ctx = context

	otelExporterUrl, exporterOk := os.LookupEnv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if !exporterOk {
		otelExporterUrl = "0.0.0.0:4317"
	}

	environment, envOk := os.LookupEnv("environment")
	if !envOk {
		environment = "development"
	}

	serviceName, nameOk := os.LookupEnv("SERVICE_NAME")
	if !nameOk {
		serviceName = "default_application"
	}

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
	if startErr := mp.Start(ctx); startErr != nil {
		err = fmt.Errorf("error starting meter provider collection; %v", startErr)
		return
	}

	return
}

func GetTraceProvider() trace.TracerProvider {
	return otel.GetTracerProvider()
	//return traceProvider
}

func GetMeterProvider() metric.MeterProvider {
	return global.GetMeterProvider()
	//return meterProvider
}

func ShutdownOpentelProviders() (err error) {
	if tpErr := traceProvider.Shutdown(ctx); tpErr != nil {
		err = tpErr
	}

	if mpErr := meterProvider.Stop(ctx); mpErr != nil {
		err = mpErr
	}
	ctx.Done()
	return
}

func initTraceProvider(exporterUrl string, serviceName string, environment string) (tp *sdktrace.TracerProvider, tpErr error) {
	//configure grpc exporter
	traceClient := otlptracegrpc.NewClient(
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(exporterUrl),
	)

	traceExp, traceExpErr := otlptrace.New(ctx, traceClient)
	if traceExpErr != nil {
		tpErr = fmt.Errorf("failed to create collector trace exporter; %v", traceExpErr)
		return
	}

	//configure trace provider resource to describe this application
	r := getAppResource(serviceName, environment)

	//create new span processor to output spans to exporter
	bsp := sdktrace.NewBatchSpanProcessor(traceExp)
	//register exporter with new trace provider
	tp = sdktrace.NewTracerProvider(
		//register span processor with trace provider
		sdktrace.WithSpanProcessor(bsp),
		//configure resource to be used in all traces from trace provider
		sdktrace.WithResource(r),
		//setup sampler to always sample traces
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	return
}

func initMeterProvider(exporterUrl string, serviceName string, environment string) (mp *controller.Controller, mpErr error) {
	//create gRPC metric client
	metricClient := otlpmetricgrpc.NewClient(
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithEndpoint(exporterUrl),
	)

	//create and start exporter based on options from metric client
	metricExp, metricExpErr := otlpmetric.New(ctx, metricClient)
	if metricExpErr != nil {
		mpErr = fmt.Errorf("failed to create the collector metrix exporter; %v", metricExpErr)
		return
	}

	r := getAppResource(serviceName, environment)

	//regitser exporter with meter provider along with other options
	mp = controller.New(
		processor.NewFactory(
			simple.NewWithHistogramDistribution(),
			metricExp,
		),
		controller.WithExporter(metricExp),
		controller.WithResource(r),
		controller.WithCollectPeriod(2*time.Second),
	)

	return
}

func getAppResource(serviceName string, environment string) *resource.Resource {
	//configure resource with optional attributes to describe application
	r, _ := resource.New(
		ctx,
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(
			// the service name used to display traces in backends
			semconv.ServiceNameKey.String(serviceName),
			attribute.String("environment", environment),
		),
	)
	return r
}
