module github.com/agenticflow/agenticflow/daemon

go 1.25.0

require (
	github.com/agenticflow/agenticflow/shared v0.0.0
	github.com/gorilla/websocket v1.5.3
	github.com/spf13/cobra v1.10.2
	pgregory.net/rapid v1.3.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
)

replace github.com/agenticflow/agenticflow/shared => ../shared
