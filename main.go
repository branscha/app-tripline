package main

import (
	"flag"
	"fmt"
	"github.com/branscha/tripline/db"
	"github.com/branscha/tripline/proc"
	"golang.org/x/crypto/ssh/terminal"
	"log"
	"os"
	"strings"
	"syscall"
)

const (
	err010 = "(tripl/010) error:%v"
	err020 = "(tripl/020) expected command: add, delete, verify, list, deleteset, copyset, listsets, sign or verifysig"
	err030 = "(tripl/030) command 'add' expects one or more filenames"
	err035 = "(tripl/035) command 'delete' expects one or more filenames"
	err040 = "(tripl/040) command 'list' does not handle arguments"
	err050 = "(tripl/050) command 'deleteset' does not parameters"
	err060 = "(tripl/060) command 'listsets' does not handle arguments"
	err070 = "(tripl/070) command 'copyset' expects a single argument, the target fileset name"
	err080 = "(tripl/080) unknown command %q"
	err090 = "(tripl/090) command 'sign' does not have parameters"
	err095 = "(tripl/095) command 'verifysig' does not have parameters"
	err100 = "(tripl/100) command read password:%v"
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
	overwrite := addFlags.Bool("overwrite", false, "Overwrite existing data if already in the database. Also see --skip.")
	filechecks := addFlags.String("filechecks", "size,modtime,ownership,permissions,sha256", "File checks.")
	dirchecks := addFlags.String("dirchecks", "child,modtime,ownership,permissions", "Directory checks.")
	skip := addFlags.Bool("skip", false, "Ignore files if already in the database. Also see --overwrite")

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

	signFlags := flag.NewFlagSet("sign/verifysig", flag.ExitOnError)
	signFileset := signFlags.String("fileset", "default", "Fileset to copy.")
	signOverwrite := signFlags.Bool("overwrite", false, "Overwrite existing signature.")

	flagSets := []*flag.FlagSet{addFlags, deleteFlags, verifyFlags, listFlags, deleteSetFlags, copySetFlags, signFlags}
	// 0 = executable name
	// 1 = command
	// 2 ... the arguments
	if len(os.Args) < 2 {
		printManualAndExit(flagSets)
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
		defer func() { must(tripDb.Rollback()) }()
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
		defer func() { must(tripDb.Rollback()) }()
		must(proc.Listsets(tripDb))
	case "copyset":
		// Parse args
		err := copySetFlags.Parse(os.Args[2:])
		if err == flag.ErrHelp {
			copySetFlags.Usage()
		}
		// Arity check
		if copySetFlags.NArg() != 1 {
			log.Fatalf(err070)
		}
		// Start writable transaction
		must(tripDb.Begin(true))
		mustCommitOrRollback(
			proc.CopySet(*copyFileset, copySetFlags.Arg(0), tripDb), tripDb)
	case "sign":
		// Parse the arguments
		err := signFlags.Parse(os.Args[2:])
		if err == flag.ErrHelp {
			signFlags.Usage()
		}
		// Arity check
		if signFlags.NArg() != 0 {
			log.Fatal(err090)
		}
		pwd, err := readSecret()
		if err != nil {
			log.Fatalf(err100, err)
		}
		// Start writable transaction
		must(tripDb.Begin(true))
		mustCommitOrRollback(proc.SignSet(*signFileset, pwd, *signOverwrite, tripDb), tripDb)
	case "verifysig":
		// Parse the arguments
		err := signFlags.Parse(os.Args[2:])
		if err == flag.ErrHelp {
			signFlags.Usage()
		}
		// Arity check
		if signFlags.NArg() != 0 {
			log.Fatal(err095)
		}
		pwd, err := readSecret()
		if err != nil {
			log.Fatalf(err100, err)
		}
		must(tripDb.Begin(false))
		defer func() { must(tripDb.Rollback()) }()
		must(proc.VerifySetSignature(*signFileset, pwd, tripDb))
	default:
		log.Printf(err080, cmd)
		printManualAndExit(flagSets)
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

// Helper to print the "usage" of each set in a list of flag sets.
func printManualAndExit(sets []*flag.FlagSet) {
	log.Printf(err020)
	for _, set := range sets {
		set.Usage()
	}
	os.Exit(1)
}

func readSecret() (string, error) {
	fmt.Print("Enter Password: ")
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", err
	}

	password := string(bytePassword)
	return strings.TrimSpace(password), nil
}
