module example

go 1.24.4

replace gitlab.citydrive.tech/back-end/go/pkg/http-client => ../

require gitlab.citydrive.tech/back-end/go/pkg/http-client v0.0.0-00010101000000-000000000000

require (
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	go.opentelemetry.io/otel v1.32.0 // indirect
	go.opentelemetry.io/otel/metric v1.32.0 // indirect
	go.opentelemetry.io/otel/trace v1.32.0 // indirect
)
