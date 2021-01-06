package proc

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/branscha/tripline/db"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var fileChecks = map[string]fileChecker{
	"nocheck":     noChecker{},
	"size":        fileSizeChecker{},
	"ownership":   ownershipChecker{},
	"content":     noChecker{},
	"modtime":     modTimeChecker{},
	"permissions": permissionsChecker{},
	"sha256":      sha256Checker{},
}

var dirChecks = map[string]fileChecker{
	"nocheck":     noChecker{},
	"ownership":   ownershipChecker{},
	"child":       childChecker{},
	"modtime":     modTimeChecker{},
	"permissions": permissionsChecker{},
}

type fileChecker interface {
	prepareCheck(fqn string, fi os.FileInfo) (interface{}, error)
	executeCheck(fqn string, data interface{}, fi os.FileInfo) error
}

const (
	err005 = "(proc/005) fileset %q underscore prefix reserved for internal use"
	err010 = "(proc/010) parse file checks:%w"
	err020 = "(proc/020) parse dir checks:%w"
	err030 = "(proc/030) unknown check %q"
	err040 = "(proc/040) file %q:%w"
	err050 = "(proc/050) file %q check %q:%w"
	err060 = "(proc/060) dir %q check %q:%w"
	err070 = "(proc/070) add file %q:%w"
	err080 = "(proc/080) list fileset %q:%w"
	err090 = "(proc/090) delete fileset %q:%w"
	err100 = "(proc/100) list filesets:%w"
	err110 = "(proc/110) copy fileset:%w"
	err120 = "(proc/120) query files %q:%w"
	err130 = "(proc/130) delete file:%w"
	err140 = "(proc/140) verify fileset %q signature:%w"
	err150 = "(proc/150) sign fileset %q:%w"
)

const (
	msg010 = "%s:basic:%v"
	msg020 = "%s:basic:file mutation"
	msg030 = "%s:basic:dir mutation"
	msg040 = "%s:%s:%v"
	msg060 = "%v:%v"
	msg070 = "skip %s"
	msg080 = "%d entries with prefix %q"
	msg085 = "%d entries"
	msg090 = "%s"
)

// Add the slice of file or directory names to the fileset. The fileset is created if it does not exist.
func AddFiles(fileNames []string, fileset string, recursive bool, overwrite bool, skip bool, filechecks string, dirchecks string, tripDb *db.TriplineDb) error {
	if strings.HasPrefix(fileset, "_") {
		log.Fatalf(err005, fileset)
	}

	fc, err := parseFileChecks(filechecks)
	if err != nil {
		log.Fatalf(err010, err)
	}
	dc, err := parseDirChecks(dirchecks)
	if err != nil {
		log.Fatalf(err020, err)
	}

	for _, fn := range fileNames {
		err := addFileOrDir(fn, fileset, recursive, overwrite, skip, fc, dc, tripDb)
		if err != nil {
			return err
		}
	}
	return nil
}

func parseFileChecks(checks string) ([]string, error) {
	fc, err := splitChecks(checks, fileChecks)
	if err != nil {
		return nil, err
	}
	return fc, err
}

func parseDirChecks(checks string) ([]string, error) {
	dc, err := splitChecks(checks, dirChecks)
	if err != nil {
		return nil, err
	}
	return dc, nil
}

// Split the string of identifiers "check1,check-2,...,check-n" into a slice and verify that each identifier
// is a valid one, it is a member of the set of valid identifiers.
func splitChecks(checks string, validSet map[string]fileChecker) ([]string, error) {
	result := strings.Split(checks, ",")
	for i, c := range result {
		result[i] = strings.ToLower(strings.TrimSpace(c))
		_, found := validSet[result[i]]
		if !found {
			return nil, fmt.Errorf(err030, result[i])
		}
	}
	return result, nil
}

func addFileOrDir(fn string, fileset string, recursive bool, overwrite bool, skip bool, filechecks []string, dirchecks []string, tripDb *db.TriplineDb) error {
	fqn, err := filepath.Abs(fn)
	if err != nil {
		return fmt.Errorf(err040, fn, err)
	}

	fi, err := os.Stat(fqn)
	if err != nil {
		return fmt.Errorf(err040, fn, err)
	}

	rec := &db.TriplineRecord{}
	rec.IsDir = fi.IsDir()
	rec.Data = make(map[string]interface{})
	if rec.IsDir {
		// It is a directory, walk over the directory checkers to collect data necessary for later verification.
		rec.Checks = dirchecks
		for _, checkName := range dirchecks {
			check, _ := dirChecks[checkName]
			checkData, err := check.prepareCheck(fqn, fi)
			if err != nil {
				// Error while producing verification data
				return fmt.Errorf(err050, fqn, checkName, err)
			}
			rec.Data[checkName] = checkData
		}
	} else {
		// It is a file, walk over the file checkers to collect data necessary for later verification.
		rec.Checks = filechecks
		for _, checkName := range filechecks {
			check, _ := fileChecks[checkName]
			checkData, err := check.prepareCheck(fqn, fi)
			if err != nil {
				// Error while producing verification data
				return fmt.Errorf(err060, fqn, checkName, err)
			}
			rec.Data[checkName] = checkData
		}
	}

	err = tripDb.AddTriplineRecord(fqn, rec, fileset, overwrite)
	if err != nil {
		if errors.Is(err, db.RecordExists) {
			if skip {
				// Ignore the error, we are skipping the files when the
				// skip flag is set.
				log.Printf(msg070, fqn)
			} else {
				// If the skip flag is not set a duplicate record results in an error
				return fmt.Errorf(err070, fqn, err)
			}
		} else {
			// An other error that has nothing to do with duplicate records.
			return fmt.Errorf(err070, fqn, err)
		}
	}

	if rec.IsDir && recursive {
		children, err := ioutil.ReadDir(fqn)
		if err != nil {
			return err
		}
		for _, child := range children {
			cfqn := filepath.Join(fqn, child.Name())
			err := addFileOrDir(cfqn, fileset, recursive, overwrite, skip, filechecks, dirchecks, tripDb)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func ListRecords(fileset string, tripDb *db.TriplineDb) error {
	if strings.HasPrefix(fileset, "_") {
		log.Fatalf(err005, fileset)
	}

	entries, err := tripDb.ListTriplineRecords(fileset)
	if err != nil {
		return fmt.Errorf(err080, fileset, err)
	}
	for _, rec := range entries {
		pretty, err := json.Marshal(rec.Record)
		if err != nil {
			// Just print the record without formatting.
			log.Printf(msg060, rec.Path, rec.Record)
		} else {
			// Here we have json formatting.
			log.Printf(msg060, rec.Path, string(pretty))
		}
	}
	return nil
}

func DeleteSet(fileset string, tripDb *db.TriplineDb) error {
	if strings.HasPrefix(fileset, "_") {
		log.Fatalf(err005, fileset)
	}

	err := tripDb.DeleteFileset(fileset)
	if err != nil {
		return fmt.Errorf(err090, fileset, err)
	}
	return nil
}

func VerifyFiles(fileNames []string, fileset string, tripDb *db.TriplineDb) (int, error) {
	if strings.HasPrefix(fileset, "_") {
		log.Fatalf(err005, fileset)
	}

	totalFails := 0
	if len(fileNames) == 0 {
		fails, err := verifyFile("", fileset, tripDb)
		if err != nil {
			return 0, err
		}
		totalFails += fails
	} else {
		for _, fn := range fileNames {
			fqn, err := filepath.Abs(fn)
			if err != nil {
				return 0, fmt.Errorf("file %q:%v", fn, err)
			}

			fails, err := verifyFile(fqn, fileset, tripDb)
			if err != nil {
				return 0, err
			}
			totalFails += fails
		}
	}
	return totalFails, nil
}

func verifyFile(fqn string, fileset string, tripDb *db.TriplineDb) (int, error) {
	entries, err := tripDb.QueryTriplineRecords(fileset, fqn)
	if err != nil {
		return 0, fmt.Errorf(err120, fqn, err)
	}

	// Report nr. of matching entries in case the user provided wrong input
	// The user can see that the input is used as a prefix which sometimes happens with options that are not spelled
	// correctly.
	if len(fqn) > 0 {
		log.Printf(msg080, len(entries), fqn)
	} else {
		log.Printf(msg085, len(entries))
	}

	fails := 0
	for _, entry := range entries {

		// Basic built-in checks
		fi, err := os.Stat(entry.Path)
		if err != nil {
			fails++
			log.Printf(msg010, entry.Path, "file not found")
			continue
		}
		if fi.IsDir() != entry.Record.IsDir {
			fails++
			if fi.IsDir() {
				log.Printf(msg020, entry.Path)
			} else {
				log.Printf(msg030, entry.Path)
			}
			continue
		}

		// user selected checks
		for _, checkName := range entry.Record.Checks {
			var checker fileChecker
			if entry.Record.IsDir {
				checker = dirChecks[checkName]
			} else {
				checker = fileChecks[checkName]
			}
			if checker == nil {
				log.Printf(msg040, entry.Path, checkName, "unknown check")
				fails++
				continue
			}
			// Execute the check.
			checkErr := checker.executeCheck(entry.Path, entry.Record.Data[checkName], fi)
			if checkErr != nil {
				log.Printf(msg040, entry.Path, checkName, checkErr)
				fails++
			}
		}
	}
	return fails, nil
}

// List the file sets in the database.
func Listsets(tripDb *db.TriplineDb) error {
	sets, err := tripDb.ListFilesets()
	if err != nil {
		return fmt.Errorf(err100, err)
	}
	for _, set := range sets {
		log.Printf(msg090, set)
	}
	return nil
}

func CopySet(from, to string, tripDb *db.TriplineDb) error {
	if strings.HasPrefix(from, "_") {
		log.Fatalf(err005, from)
	}

	if strings.HasPrefix(to, "_") {
		log.Fatalf(err005, to)
	}

	err := tripDb.CopyFileset(from, to)
	if err != nil {
		return fmt.Errorf(err110, err)
	}
	return nil
}

func DeleteFiles(fileNames []string, fileset string, tripDb *db.TriplineDb) error {
	if strings.HasPrefix(fileset, "_") {
		log.Fatalf(err005, fileset)
	}

	for _, fn := range fileNames {
		fqn, err := filepath.Abs(fn)
		if err != nil {
			return fmt.Errorf(err040, fn, err)
		}

		entries, err := tripDb.QueryTriplineRecords(fileset, fqn)
		if err != nil {
			return fmt.Errorf(err120, fqn, err)
		}

		for _, entry := range entries {
			err := tripDb.DeleteTriplineRecord(entry.Path, fileset, true)
			if err != nil {
				return fmt.Errorf(err130, entry.Path)
			}
		}
	}
	return nil
}

func SignSet(fileset string, password string, update bool, tripDb *db.TriplineDb) error {
	if strings.HasPrefix(fileset, "_") {
		log.Fatalf(err005, fileset)
	}
	err := tripDb.SignFileset(fileset, password, update)
	if err != nil {
		return fmt.Errorf(err150, fileset, err)
	}
	return nil
}

func VerifySetSignature(fileset string, password string, tripDb *db.TriplineDb) error {
	if strings.HasPrefix(fileset, "_") {
		log.Fatalf(err005, fileset)
	}

	err := tripDb.VerifyFilesetSignature(fileset, password)
	if err != nil {
		return fmt.Errorf(err140, fileset, err)
	}
	return nil
}
