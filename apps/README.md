# apps/

Entrypoints. Each subfolder is a deployable unit (HTTP API, worker, CLI, web app).

Apps **wire** together libs — they should contain almost no business logic. If
you find yourself writing logic here, move it into the matching domain in `libs/`.

Apps depend on libs. Libs never depend on apps.
