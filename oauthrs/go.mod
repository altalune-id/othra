module github.com/altalune-id/othra/oauthrs

go 1.26.10

require (
	github.com/altalune-id/othra/oidcrp v0.0.0
	github.com/go-jose/go-jose/v4 v4.1.6
	github.com/stretchr/testify v1.12.1
)

require (
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sync v0.24.0 // indirect
)

replace github.com/altalune-id/othra/oidcrp => ../oidcrp
