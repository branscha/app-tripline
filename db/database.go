package db

import (
   "encoding/json"
   "errors"
   "fmt"
   "github.com/boltdb/bolt"
   "os"
   "path"
   "strings"
)

const (
   dbname = ".tripline"
)

const (
   err010 = "(db/010) create/open fileset '%s':%v"
   err020 = "(db/020) unknown fileset '%s'"
   err030 = "(db/030) marshal tripline record:%v"
   err040 = "(db/040) add tripline record to database:%v"
   err050 = "(db/050) path '%s' does not exist in fileset '%s'"
   err060 = "(db/060) delete tripline record:%v"
   err070 = "(db/070) unmarshal tripline record:%v"
   err080 = "(db/080) transaction required"
   err085 = "(db/085) write transaction required"
   err090 = "(db/090) nested transaction"
   err100 = "(db/100) transaction forbidden"
   err110 = "(db/110) create fileset '%s':%v"
   err120 = "(db/120) copy fileset '%s':%v"
)

var (
   RecordExists = errors.New("(db/005) record exists")
)

// Record to store in the tripline database.
type TriplineRecord struct {
   IsDir bool `json:"isDir"`
   Checks []string `json:"checks"`
   Data map[string]interface{} `json:"data"`
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
func (db *TriplineDb) QueryTriplineRecords(fileset string, pathPrefix string) ([]TriplineEntry, error) {
   if db.boltTx == nil  {
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
   if db.boltTx == nil  {
      return nil, fmt.Errorf(err080)
   }
   result := make([]string, 0)
   err := db.boltTx.ForEach(func(name []byte, _ *bolt.Bucket) error {
      result = append(result, string(name))
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

func (db *TriplineDb) CopyFileset(src, target string) error {
   if db.boltTx == nil  {
      return fmt.Errorf(err080)
   }

   // Dig up the source bucket
   srcBkt := db.boltTx.Bucket([]byte(src))
   if srcBkt == nil {
      return fmt.Errorf(err020, src)
   }

   // Create target buket
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
