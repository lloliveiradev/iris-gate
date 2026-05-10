module github.com/iris-gate/iris-gate/libs/payments

go 1.25.0

toolchain go1.25.10

require (
	github.com/go-chi/chi/v5 v5.2.5
	github.com/google/uuid v1.6.0
	github.com/iris-gate/iris-gate/libs/contracts/generated/go v0.0.0-00010101000000-000000000000
	github.com/iris-gate/iris-gate/libs/identity v0.0.0-00010101000000-000000000000
	github.com/jackc/pgx/v5 v5.9.2
	github.com/oapi-codegen/runtime v1.4.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/text v0.37.0 // indirect
)

replace (
	github.com/iris-gate/iris-gate/libs/contracts/generated/go => ../contracts/generated/go
	github.com/iris-gate/iris-gate/libs/identity => ../identity
)
