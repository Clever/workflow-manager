module github.com/Clever/workflow-manager

go 1.16

require (
	github.com/Clever/aws-sdk-go-counter v1.10.39-0.20190610193009-b603aedc6d67
	github.com/Clever/go-process-metrics v0.4.0
	github.com/Clever/kayvee-go/v7 v7.7.0
	github.com/Clever/launch-gen v0.0.0-20230222233441-17c275320509
	github.com/Clever/wag/clientconfig/v9 v9.0.0-20230222234634-ddf6a6175e43
	github.com/Clever/workflow-manager/gen-go/client v0.0.0-00010101000000-000000000000
	github.com/Clever/workflow-manager/gen-go/models v0.0.0-00010101000000-000000000000
	github.com/aws/aws-lambda-go v1.34.1
	github.com/aws/aws-sdk-go v1.44.89
	github.com/elastic/go-elasticsearch/v6 v6.8.10
	github.com/ghodss/yaml v1.0.0
	github.com/go-errors/errors v1.1.1
	github.com/go-openapi/strfmt v0.21.2
	github.com/go-openapi/swag v0.21.1
	github.com/golang/mock v1.6.0
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/golang-lru v0.5.4
	github.com/kardianos/osext v0.0.0-20170510131534-ae77be60afb1
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826
	github.com/satori/go.uuid v1.2.0
	github.com/stretchr/testify v1.8.0
	go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux v0.34.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.35.0
	go.opentelemetry.io/otel v1.10.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.10.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.10.0
	go.opentelemetry.io/otel/sdk v1.10.0
	go.opentelemetry.io/otel/trace v1.10.0
	golang.org/x/sync v0.0.0-20220601150217-0de741cfad7f
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba
	golang.org/x/xerrors v0.0.0-20220609144429-65e65417b02f
)

replace github.com/Clever/workflow-manager/gen-go/models => ./gen-go/models

replace github.com/Clever/workflow-manager/gen-go/client => ./gen-go/client
