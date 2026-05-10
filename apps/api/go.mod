module github.com/iris-gate/iris-gate/apps/api

go 1.24

// O módulo de contratos é resolvido localmente via go.work na raiz.
// Versão placeholder + replace permitem build fora do workspace também.
require (
	github.com/go-chi/chi/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/iris-gate/iris-gate/libs/contracts/generated/go v0.0.0-00010101000000-000000000000
	github.com/oapi-codegen/runtime v1.4.0
)

replace github.com/iris-gate/iris-gate/libs/contracts/generated/go => ../../libs/contracts/generated/go
