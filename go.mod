module xyzw_study

go 1.23

require (
	github.com/husanpao/game-mitm v0.0.0-20250310071321-f2c191024c8a
	github.com/pierrec/lz4 v2.6.1+incompatible
)

replace github.com/husanpao/game-mitm => ./third_party/game-mitm

require (
	github.com/fatih/color v1.18.0 // indirect
	github.com/frankban/quicktest v1.14.6 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/sys v0.25.0 // indirect
)
