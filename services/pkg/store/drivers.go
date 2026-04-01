//go:build cgo_sqlite || postgres_driver

package store

// Import database drivers for side-effect registration with database/sql.
// Build with -tags=cgo_sqlite,postgres_driver to enable these imports,
// or register drivers manually in your application's main package:
//
//	import _ "github.com/lib/pq"
//	import _ "modernc.org/sqlite"
//
// This file is excluded from default builds to avoid hard dependencies on
// external driver packages. The store implementations use only database/sql
// interfaces and work with any compatible driver.

// When building with the cgo_sqlite tag:
//   import _ "modernc.org/sqlite"
//
// When building with the postgres_driver tag:
//   import _ "github.com/lib/pq"
