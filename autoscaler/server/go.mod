module server

go 1.21.9

require logging v0.0.0

require (
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/olekukonko/tablewriter v0.0.5 // indirect
)

replace logging => ../logging
