package main

import (
	"flag"
	"github.com/branscha/tripline/db"
	"github.com/branscha/tripline/proc"
	"log"
	"os"
)

const (
	err010 = "(tripl/010) error:%v"
	err020 = "(tripl/020) expected command: add, delete, verify, list, deleteset, copyset or listsets."
	err030 = "(tripl/030) command 'add' expects one or more filenames."
	err035 = "(tripl/035) command 'delete' expects one or more filenames."
	err040 = "(tripl/040) command 'list' does not handle arguments."
	err050 = "(tripl/050) command 'deleteset' does not handle arguments."
	err060 = "(tripl/060) command 'listsets' does not handle arguments."
	err070 = "(tripl/070) command 'copyset' expects a single argument, the target fileset name."
	err080 = "(tripl/080) unknown command '%s'"
)

const (
	msg010 = "%d failed checks"
	msg020 = "0 failed checks"
)

func main() {
	// Remove timestamps from the default logger.
	log.SetFlags(0)

	// Define command line args
	addFlags := flag.NewFlagSet("add", flag.ExitOnError)
	addFileset := addFlags.String("fileset", "default", "Fileset where files are added. Created if not present.")
	recursive := addFlags.Bool("recursive", true, "Add directories recursively.")
	overwrite := addFlags.Bool("overwrite", false, "Overwrite existing data.")
	filechecks := addFlags.String("filechecks", "size,modtime,ownership,permissions,sha256", "File checks.")
	dirchecks := addFlags.String("dirchecks", "child,modtime,ownership,permissions", "Directory checks.")
	skip := addFlags.Bool("skip", false, "Skip existing data.")

	deleteFlags := flag.NewFlagSet("delete", flag.ExitOnError)
	deleteFileset := deleteFlags.String("fileset", "default", "Fileset where files will be deleted.")

	verifyFlags := flag.NewFlagSet("verify", flag.ExitOnError)
	verifyFileset := verifyFlags.String("fileset", "default", "Fileset containing the checks.")

	listFlags := flag.NewFlagSet("list", flag.ExitOnError)
	listFileset := listFlags.String("fileset", "default", "Fileset for which contents is listed.")

	deleteSetFlags := flag.NewFlagSet("deleteset", flag.ExitOnError)
	deleteSetFileset := deleteSetFlags.String("fileset", "default", "Fileset to delete.")

	copySetFlags := flag.NewFlagSet("copyset", flag.ExitOnError)
	copyFileset := copySetFlags.String("fileset", "default", "Fileset to copy.")

	// 0 = executable name
	// 1 = command
	// 2 ... the arguments
	if len(os.Args) < 2 {
		log.Fatalf(err020)
	}
	cmd := os.Args[1]

	// Open the database + make sure it will be closed.
	tripDb, err := db.OpenDefaultTriplineDb()
	must(err)
	defer func() { must(tripDb.Close()) }()

	switch cmd {
	case "add":
		// Parse the arguments
		err := addFlags.Parse(os.Args[2:])
		if err == flag.ErrHelp {
			addFlags.Usage()
		}
		// Arity check
		if addFlags.NArg() <= 0 {
			log.Fatal(err030)
		}
		// Start writable transaction
		must(tripDb.Begin(true))
		mustCommitOrRollback(
			proc.AddFiles(addFlags.Args(), *addFileset, *recursive, *overwrite, *skip, *filechecks, *dirchecks, tripDb), tripDb)
	case "delete":
		// Parse the arguments
		err := deleteFlags.Parse(os.Args[2:])
		if err == flag.ErrHelp {
			deleteFlags.Usage()
		}
		// Arity check
		if deleteFlags.NArg() <= 0 {
			log.Fatal(err035)
		}
		// Start writable transaction
		must(tripDb.Begin(true))
		mustCommitOrRollback(
			proc.DeleteFiles(deleteFlags.Args(), *deleteFileset, tripDb), tripDb)
	case "verify":
		// Parse arguments
		err := verifyFlags.Parse(os.Args[2:])
		if err == flag.ErrHelp {
			verifyFlags.Usage()
		}
		// Start read transaction
		must(tripDb.Begin(false))
		defer func() {must(tripDb.Rollback()) }()
		fails, err := proc.VerifyFiles(verifyFlags.Args(), *verifyFileset, tripDb)
		must(err)
		if fails > 0 {
			// If there are failed checks, the command should exit with non-zero exit code as well.
			// There is a difference in how to handle failures and success here.
			log.Fatalf(msg010, fails)
		} else {
			// If there are no failures, the command should exit with code 0.
			log.Println(msg020)
		}
	case "list":
		// Parse args
		err := listFlags.Parse(os.Args[2:])
		if err == flag.ErrHelp {
			listFlags.Usage()
		}
		// Arity check
		if flag.NArg() > 1 {
			log.Fatalf(err040)
		}
		// Start readable transaction
		must(tripDb.Begin(false))
		defer func() { must(tripDb.Rollback()) }()
		must(proc.ListRecords(*listFileset, tripDb))
	case "deleteset":
		// Parse args
		err := deleteSetFlags.Parse(os.Args[2:])
		if err == flag.ErrHelp {
			deleteSetFlags.Usage()
		}
		// Arity check
		if deleteSetFlags.NArg() > 0 {
			log.Fatal(err050)
		}
		// Start writable transaction
		must(tripDb.Begin(true))
		mustCommitOrRollback(
			proc.DeleteSet(*deleteSetFileset, tripDb), tripDb)
	case "listsets":
		// Arity check
		if len(os.Args) > 2 {
			log.Fatalf(err060)
		}
		// Start readable transaction
		must(tripDb.Begin(false))
		defer func() { must(tripDb.Rollback())}()
		must(proc.Listsets(tripDb))
	case "copyset":
		// Parse args
		err := copySetFlags.Parse(os.Args[2:])
		if err == flag.ErrHelp {
			copySetFlags.Usage()
		}
		// Arity check
		if copySetFlags.NArg() != 1  {
			log.Fatalf(err070)
		}
		// Start writable transaction
		must(tripDb.Begin(true))
		defer func() {must(tripDb.Rollback())}()
		mustCommitOrRollback(proc.CopySet(*copyFileset, copySetFlags.Arg(0), tripDb), tripDb)
	default:
		log.Fatalf(err080, cmd)
	}
}

// Helper for database termination operations.
// Make sure the error does not go unhandled, write to the log file and exit.
func must(err error) {
	if err != nil {
		log.Fatalf(err010, err)
	}
}

// Helper to commit/rollback the database according to the result of the operation.
// The idea is to insert the transactional call in the first argument.
func mustCommitOrRollback(err error, tripDb *db.TriplineDb) {
	if err == nil {
		// No errors, we can commit the changes.
		must(tripDb.Commit())
	} else {
		// Roll back all database modifications if an error was reported.
		must(tripDb.Rollback())
		// Print the message and terminate with an error.
		log.Fatal(err)
	}
}


