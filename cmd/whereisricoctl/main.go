// Command whereisricoctl is the operator CLI for whereisrico: ingesting new
// GPS data into the database, and one-time migration from the old Python
// system.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "ingest":
		err = runIngest(os.Args[2:])
	case "import-legacy":
		err = runImportLegacy(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "whereisricoctl: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "whereisricoctl:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: whereisricoctl <command> [flags]

commands:
  ingest         scan a directory for .gpx / .textproto files and load new/changed ones into the database
  import-legacy  import coords/trips from the old Python system's sqlite database

Run "whereisricoctl <command> -h" for flags on a specific command.`)
}
