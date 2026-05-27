module github.com/agenticflow/agenticflow/server

go 1.25.0

require (
	github.com/agenticflow/agenticflow/shared v0.0.0
	github.com/go-chi/chi/v5 v5.3.0
	github.com/go-chi/cors v1.2.2
	github.com/gorilla/websocket v1.5.3
	github.com/jackc/pgx/v5 v5.9.2
	golang.org/x/crypto v0.52.0
	pgregory.net/rapid v1.3.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/text v0.37.0 // indirect
)

replace github.com/agenticflow/agenticflow/shared => ../shared
