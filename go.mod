module github.com/eshaffer321/monarchmoney-sync-backend

go 1.24.0

replace github.com/costco-go => /Users/erickshaffer/code/costco-go

replace (
	github.com/eshaffer321/monarchmoney-go => /Users/erickshaffer/code/monarchmoney-go
	github.com/eshaffer321/walmart-client => /Users/erickshaffer/code/walmart-api/walmart-client
)

require (
	github.com/costco-go v0.0.0
	github.com/eshaffer321/monarchmoney-go v0.0.0
	github.com/eshaffer321/walmart-client v0.0.0-00010101000000-000000000000
	github.com/gin-contrib/cors v1.7.6
	github.com/gin-gonic/gin v1.10.1
	github.com/google/uuid v1.6.0
	github.com/mattn/go-sqlite3 v1.14.32
	github.com/stretchr/testify v1.11.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/bytedance/sonic v1.13.3 // indirect
	github.com/bytedance/sonic/loader v0.2.4 // indirect
	github.com/cloudwego/base64x v0.1.5 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.9 // indirect
	github.com/getsentry/sentry-go v0.35.1 // indirect
	github.com/gin-contrib/sse v1.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.26.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.5 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.0 // indirect
	golang.org/x/arch v0.18.0 // indirect
	golang.org/x/crypto v0.39.0 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)
