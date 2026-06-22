module github.com/Ph4wkm00n/IronGolem_OS/services

go 1.25.0

// v0.2 Step 1: bring the connectors module into the gateway's import graph so
// real connector instances (starting with Telegram) can be registered into the
// connector pump. The connectors module stays a separate Go module so the
// connectors track can publish independently — the replace directive
// short-circuits module resolution to the sibling checkout for CI + dev.
replace github.com/Ph4wkm00n/IronGolem_OS/connectors => ../connectors

require (
	github.com/Ph4wkm00n/IronGolem_OS/connectors v0.0.0-00010101000000-000000000000
	github.com/google/uuid v1.6.0
	modernc.org/sqlite v1.53.0
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/sys v0.44.0 // indirect
	modernc.org/libc v1.73.4 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)
