module github.com/iris-gate/iris-gate/libs/payments

go 1.27.0

toolchain go1.27.4

require (
	github.com/go-chi/chi/v5 v5.5.0
	github.com/google/uuid v1.6.0
	github.com/iris-gate/iris-gate/libs/contracts/generated/go v0.0.0-00010101000000-000000000000
	github.com/iris-gate/iris-gate/libs/identity v0.0.0-00010101000000-000000000000
	github.com/jackc/pgx/v5 v5.12.0
	github.com/oapi-codegen/runtime v1.8.0
	github.com/segmentio/kafka-go v0.4.51
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	github.com/klauspost/compress v1.20.4 // indirect
	github.com/pierrec/lz4/v4 v4.1.33 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/text v0.44.0 // indirect
)

replace (
	github.com/iris-gate/iris-gate/libs/contracts/generated/go => ../contracts/generated/go
	github.com/iris-gate/iris-gate/libs/identity => ../identity
)
