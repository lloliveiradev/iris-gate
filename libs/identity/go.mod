module github.com/iris-gate/iris-gate/libs/identity

go 1.27.0

toolchain go1.27.4

require (
	github.com/go-chi/chi/v5 v5.5.0
	github.com/google/uuid v1.6.0
	github.com/iris-gate/iris-gate/libs/contracts/generated/go v0.0.0-00010101000000-000000000000
	github.com/jackc/pgx/v5 v5.12.0
	github.com/oapi-codegen/runtime v1.8.0
	golang.org/x/crypto v0.58.0
)

// Resolved locally through go.work + replace (same pattern as apps/api).
replace github.com/iris-gate/iris-gate/libs/contracts/generated/go => ../contracts/generated/go

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/text v0.44.0 // indirect
)
