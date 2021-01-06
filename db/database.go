package db

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/branscha/tripline/crypto"
	"log"
	"os"
	"path"
	"strings"
)

const (
	dbname    = ".tripline"
	sigbucket = "_signatures"
)

const (
	err005 = "(db/005) record exists"
	err010 = "(db/010) create/open fileset %q:%w"
	err020 = "(db/020) unknown fileset %q"
	err030 = "(db/030) marshal tripline record:%w"
	err040 = "(db/040) add tripline record to database:%w"
	err050 = "(db/050) path %q does not exist in fileset %q"
	err060 = "(db/060) delete tripline record:%w"
	err070 = "(db/070) unmarshal tripline record:%w"
	err080 = "(db/080) transaction required"
	err085 = "(db/085) write transaction required"
	err090 = "(db/090) nested transaction"
	err100 = "(db/100) transaction forbidden"
	err110 = "(db/110) create fileset %q:%w"
	err120 = "(db/120) copy fileset %q:%w"
	err130 = "(db/130) open/create signatures:%w"
	err140 = "(db/140) fileset signature %q exists"
	err150 = "(db/150) sign fileset %q:%w"
	err160 = "(db/160) fileset hash %q:%w"
	err170 = "(db/170) no signatures, none added or tampered"
	err180 = "(db/180) no signature, not added or tampered"
	err190 = "(db/190) wrong password or tampered: %w"
	err200 = "(db/200) contents changed or tampered"
)

var (
	RecordExists = errors.New(err005)
)

// Record to store in the tripline database.
type TriplineRecord struct {
	IsDir  bool                   `json:"isDir"`
	Checks []string               `json:"checks"`
	Data   map[string]interface{} `json:"data"`
}

type TriplineEntry struct {
	Record TriplineRecord
	Path   string
}

type TriplineDb struct {
	boltDb *bolt.DB
	boltTx *bolt.Tx
}

// Open the Tripline database in the default location.
// Normally it is the users home directory.
func OpenDefaultTriplineDb() (*TriplineDb, error) {
	// Construct the path to the tripline database to be
	// ${HOME}/.tripline
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dbPath := path.Join(home, dbname)
	// Open/create the database.
	return OpenTriplineDb(dbPath)
}

// Open the Tripline database in the default location.
// Normally it is the users home directory.
func OpenTriplineDb(dbPath string) (*TriplineDb, error) {
	// Open/create the bolt database.
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, err
	}
	return &TriplineDb{db, nil}, nil
}

func (db *TriplineDb) Begin(write bool) error {
	if db.boltTx != nil {
		return fmt.Errorf(err090)
	}
	tx, err := db.boltDb.Begin(write)
	if err != nil {
		return err
	}
	db.boltTx = tx
	return nil
}

func (db *TriplineDb) Commit() error {
	if db.boltTx == nil {
		return fmt.Errorf(err080)
	}
	err := db.boltTx.Commit()
	// Whatever the outcome, remove the transaction
	db.boltTx = nil
	if err != nil {
		return err
	}
	return nil
}

func (db *TriplineDb) Rollback() error {
	if db.boltTx == nil {
		return fmt.Errorf(err080)
	}
	err := db.boltTx.Rollback()
	// Whatever the outcome, remove the transaction.
	db.boltTx = nil
	if err != nil {
		return err
	}
	return nil
}

// Close the tripline database.
// It is necessary to close the database.
func (db *TriplineDb) Close() error {
	if db.boltTx != nil {
		return fmt.Errorf(err100)
	}
	if db.boltDb != nil {
		return db.boltDb.Close()
	}
	return nil
}

// Check if the tripline database contains a record associated with the path in the fileset.
// Returns an error if the fileset does not exist.
// Returns a boolean if the fileset exists.
func (db *TriplineDb) HasTriplineRecord(path, fileset string) (bool, error) {
	var hasTriplineRecord = false
	err := db.boltDb.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(fileset))
		if bkt == nil {
			return fmt.Errorf(err020, fileset)
		}
		hasTriplineRecord = nil != bkt.Get([]byte(path))
		return nil
	})
	return hasTriplineRecord, err
}

// Add a new record to the tripline database.
// Returns an error if the record already exists, except if the overwrite flag is set, in that case the existing record will
// be overwritten. The fileset is automatically created if it does not yet exists.
func (db *TriplineDb) AddTriplineRecord(path string, rec *TriplineRecord, fileset string, overwrite bool) error {
	if db.boltTx == nil || !db.boltTx.Writable() {
		return fmt.Errorf(err085)
	}
	// Create a json version of the record.
	jsn, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf(err030, err)
	}

	bkt, err := db.boltTx.CreateBucketIfNotExists([]byte(fileset))
	if err != nil {
		return fmt.Errorf(err010, fileset, err)
	}

	key := []byte(path)
	// If the path already exists and we are not forcing, we have an error.
	// By default we do not overwrite existing entries.
	if (bkt.Get(key) != nil) && !overwrite {
		return RecordExists
	}

	// Write the entry to the database.
	err = bkt.Put(key, []byte(jsn))
	if err != nil {
		return fmt.Errorf(err040, err)
	}

	return nil
}

// Delete a record from the tripline database.
// Returns an error if the database does not contain the record, except when the skip flag is set, then the function
// will always succeed.
func (db *TriplineDb) DeleteTriplineRecord(path string, fileset string, skip bool) error {
	if db.boltTx == nil || !db.boltTx.Writable() {
		return fmt.Errorf(err085)
	}

	bkt := db.boltTx.Bucket([]byte(fileset))
	if bkt == nil {
		if skip {
			return nil
		} else {
			return fmt.Errorf(err020, fileset)
		}
	}

	key := []byte(path)

	// If the path already exists and we are not forcing, we have an error.
	// By default we do not overwrite existing entries.
	if (bkt.Get(key) == nil) && !skip {
		return fmt.Errorf(err050, path, fileset)
	}

	err := bkt.Delete(key)
	if err != nil {
		return fmt.Errorf(err060, err)
	}
	return nil
}

// List the contents of a fileset.
// Returns an error if the fileset does not exist.
func (db *TriplineDb) ListTriplineRecords(fileset string) ([]TriplineEntry, error) {
	return db.QueryTriplineRecords(fileset, "")
}

// List the contents of a fileset, return the entries that match the given path prefix.
// Returns an error if the fileset does not exist.
// This is an easy way to query the subdirectories an files when the prefix is a directory path.
func (db *TriplineDb) QueryTriplineRecords(fileset string, pathPrefix string) ([]TriplineEntry, error) {
	if db.boltTx == nil {
		return nil, fmt.Errorf(err080)
	}

	result := make([]TriplineEntry, 0)

	// Dig up the bucket
	bkt := db.boltTx.Bucket([]byte(fileset))
	if bkt == nil {
		return nil, fmt.Errorf(err020, fileset)
	}
	// Loop over the bucket
	c := bkt.Cursor()
	for k, v := c.First(); k != nil; k, v = c.Next() {
		p := string(k)
		if strings.HasPrefix(p, pathPrefix) {
			entry := &TriplineEntry{}
			entry.Path = p
			err := json.Unmarshal(v, &entry.Record)
			if err != nil {
				return nil, fmt.Errorf(err070, err)
			}
			result = append(result, *entry)
		}
	}
	return result, nil
}

// List the filesets in the tripline database.
func (db *TriplineDb) ListFilesets() ([]string, error) {
	if db.boltTx == nil {
		return nil, fmt.Errorf(err080)
	}
	result := make([]string, 0)
	err := db.boltTx.ForEach(func(name []byte, _ *bolt.Bucket) error {
		bucketName := string(name)
		// Bucket names starting with underscores are reserved names for internal use.
		// Example _signatures bucket to store the fileset signatures.
		if !strings.HasPrefix(bucketName, "_") {
			result = append(result, bucketName)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Delete a fileset from teh tripline database.
// Returns an error if the fileset does not exist.
func (db *TriplineDb) DeleteFileset(fileset string) error {
	if db.boltTx == nil || !db.boltTx.Writable() {
		return fmt.Errorf(err085)
	}

	bkt := db.boltTx.Bucket([]byte(fileset))
	if bkt == nil {
		return fmt.Errorf(err020, fileset)
	}
	return db.boltTx.DeleteBucket([]byte(fileset))
}

// Copy the contents of an existing fileset to a new fileset with a new name.
// The existing fileset must exist, the new fileset should not yet exist.
func (db *TriplineDb) CopyFileset(src, target string) error {
	if db.boltTx == nil || !db.boltTx.Writable() {
		return fmt.Errorf(err085)
	}

	// Dig up the source bucket
	srcBkt := db.boltTx.Bucket([]byte(src))
	if srcBkt == nil {
		return fmt.Errorf(err020, src)
	}

	// Create target bucket
	targetBkt, err := db.boltTx.CreateBucket([]byte(target))
	if err != nil {
		return fmt.Errorf(err110, target, err)
	}

	// Loop over the bucket
	c := srcBkt.Cursor()
	for k, v := c.First(); k != nil; k, v = c.Next() {
		err := targetBkt.Put(k, v)
		if err != nil {
			return fmt.Errorf(err120, target, err)
		}
	}
	return nil
}

// Create a signature of the fileset contents and store it in a special _signatures bucket.
func (db *TriplineDb) SignFileset(fileset string, password string, update bool) error {
	if db.boltTx == nil || !db.boltTx.Writable() {
		return fmt.Errorf(err085)
	}

	// Fetch the signature bucket. Or create it if it does not yet exists.
	signaturesBkt, err := db.boltTx.CreateBucketIfNotExists([]byte(sigbucket))
	if err != nil {
		return fmt.Errorf(err130, err)
	}

	// Fetch the signature.
	// The user has to explicitly overwrite the signature using the --overwrite option.
	oldSignature := signaturesBkt.Get([]byte(fileset))
	if oldSignature != nil && !update {
		return fmt.Errorf(err140, fileset)
	}

	// Dig up the fileset bucket.
	srcBkt := db.boltTx.Bucket([]byte(fileset))
	if srcBkt == nil {
		return fmt.Errorf(err020, fileset)
	}

	// Calculate fileset bucket hash.
	hash, err := calcBucketHash(srcBkt)
	if err != nil {
		return err
	}
	log.Printf("hash: %x", hash)

	// Calculate the signature using the filest bucket contents.
	signature, err := crypto.Encrypt([]byte(password), hash)
	if err != nil {
		return fmt.Errorf(err150, fileset, err)
	}
	log.Printf("signature: %x", signature)

	// Store the signature in the _signatures bucket.
	signaturesBkt.Put([]byte(fileset), signature)
	return nil
}

// Verify if the validity of the existing fileset signature.
// First we decrypt the signature and compare the hash that was calculated at the time of signing to the current hash.
// If any intermediary steps fail the process fails, it might be the result of tampering.
func (db *TriplineDb) VerifyFilesetSignature(fileset string, password string) error {
	if db.boltTx == nil {
		return fmt.Errorf(err080)
	}

	// Dig up the fileset bucket.
	srcBkt := db.boltTx.Bucket([]byte(fileset))
	if srcBkt == nil {
		return fmt.Errorf(err020, fileset)
	}

	// Calculate the actual bucket hash.
	hash, err := calcBucketHash(srcBkt)
	if err != nil {
		return fmt.Errorf(err160, fileset, err)
	}

	// Fetch the signature bucket.
	// An attacker might have removed the bucket it might indicate tampering.
	// If the user never created a signature, the bucket does not exist either.
	signaturesBkt := db.boltTx.Bucket([]byte(sigbucket))
	if signaturesBkt == nil {
		return fmt.Errorf(err170)
	}

	// Fetch the signature.
	// An attacker might have removed the fileset's signature. It might indicate tampering.
	// The user might never have created a signature for the fileset.
	oldSignature := signaturesBkt.Get([]byte(fileset))
	if oldSignature == nil {
		return fmt.Errorf(err180)
	}

	// The old hash cannot be reconstructed from the signature.
	// An attacker might have replaced the signature with another one.
	// The user might have forgotten the password.
	plain, err := crypto.Decrypt([]byte(password), oldSignature)
	if err != nil {
		return fmt.Errorf(err190, err)
	}

	// Compare the old hash from the signature with the newly calculated one.
	// The fileset might be tampered.
	// The user might have changed the fileset without creating a new signature.
	if bytes.Compare(plain, hash) != 0 {
		return fmt.Errorf(err200)
	}

	log.Printf("Integrity fileset %q is ok.", fileset)
	return nil
}

// Calculate sha256 of the contents of a bucket. Both keys and values are taken into account.
func calcBucketHash(srcBkt *bolt.Bucket) ([]byte, error) {
	h := sha256.New()
	c := srcBkt.Cursor()
	for k, v := c.First(); k != nil; k, v = c.Next() {
		_, err := h.Write(k)
		if err != nil {
			return nil, err
		}
		_, err = h.Write(v)
		if err != nil {
			return nil, err
		}
	}
	return h.Sum(nil), nil
}
