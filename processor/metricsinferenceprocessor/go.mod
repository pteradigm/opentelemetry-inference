module github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor

go 1.23.0

toolchain go1.23.9

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.114.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.114.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.32.1-0.20250513225039-2c5086381935
	go.opentelemetry.io/collector/component/componenttest v0.126.1-0.20250513225039-2c5086381935
	go.opentelemetry.io/collector/confmap v1.32.1-0.20250513225039-2c5086381935
	go.opentelemetry.io/collector/consumer v1.32.1-0.20250513225039-2c5086381935
	go.opentelemetry.io/collector/consumer/consumertest v0.126.1-0.20250513225039-2c5086381935
	go.opentelemetry.io/collector/featuregate v1.32.1-0.20250513225039-2c5086381935
	go.opentelemetry.io/collector/pdata v1.32.1-0.20250513225039-2c5086381935
	go.opentelemetry.io/collector/processor v1.32.1-0.20250513225039-2c5086381935
	go.opentelemetry.io/collector/processor/processortest v0.126.1-0.20250513225039-2c5086381935
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
	google.golang.org/grpc v1.72.0
	google.golang.org/protobuf v1.36.6
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.114.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.126.1-0.20250513225039-2c5086381935 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.126.1-0.20250513225039-2c5086381935 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.126.1-0.20250513225039-2c5086381935 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.126.1-0.20250513225039-2c5086381935 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.126.1-0.20250513225039-2c5086381935 // indirect
	go.opentelemetry.io/collector/pipeline v0.126.1-0.20250513225039-2c5086381935 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.126.1-0.20250513225039-2c5086381935 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.10.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/log v0.11.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)
