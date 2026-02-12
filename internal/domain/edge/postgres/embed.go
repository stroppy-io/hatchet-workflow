package postgres

import _ "embed"

//go:embed embed/postgres-docker-compose.yaml
var PostgresDockerCompose []byte

//go:embed embed/patroni.Dockerfile
var PatroniDockerfile []byte

//go:embed embed/patroni-entrypoint.sh
var PatroniEntrypoint []byte
