# go-opentel
Go package for configuring opentelemetry meter and trace providers

Package configures gRPC exporter with the defined exporter url (otelExporterUrl)

##ENV Vars
The package uses the following env vars to setup the opentelemetry providers
* `environment` - environment of the application
* `SERVICE_NAME` - name that will be use to identify the service/application
* `OTEL_EXPORTER_OTLP_ENDPOINT` - the IP of the otlp exporter
