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
	github.com/mattn/go-sqlite3 v1.14.32
	github.com/stretchr/testify v1.11.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/getsentry/sentry-go v0.36.0 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.30.0 // indirect
)
