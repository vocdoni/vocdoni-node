package mongodb

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/log"
)

const (
	// MongodbTimeoutConnect is the timeout for connecting to the database
	MongodbTimeoutConnect = 10 * time.Second
	// MongodbTimeoutCommit is the timeout for committing a batch transaction
	MongodbTimeoutCommit = 12 * time.Second
	// MongodbTimeoutQuery is the timeout for querying the database
	MongodbTimeoutQuery = 4 * time.Second
)

// MongoDB is a MongoDB implementation of the db.DB interface
type MongoDB struct {
	db         *mongo.Database
	collection string
}

type KeyVal struct {
	Key   string `bson:"_id" json:"key"` // needs to be an ugly string because of regex
	Value []byte `bson:"value" json:"value"`
}

func New(opts db.Options) (*MongoDB, error) {
	url := os.Getenv("MONGODB_URL")

	if url == "" {
		return nil, fmt.Errorf("missing MONGODB_URL env var")
	}
	// we use the path as the collection name (and we hash it to avoid invalid chars)
	collection := fmt.Sprintf("%x", sha256.Sum256([]byte(opts.Path)))[:12]
	log.Debugw("connecting to mongo database", "url", url, "collection", collection)
	clientOptions := options.Client().ApplyURI(url).SetMaxConnecting(20)
	ctx, cancel := context.WithTimeout(context.Background(), MongodbTimeoutConnect)
	defer cancel()
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}
	database := client.Database(collection)

	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		return nil, fmt.Errorf("cannot connect to mongodb: %w", err)
	}

	return &MongoDB{db: database, collection: collection}, nil
}

func (d *MongoDB) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return d.db.Client().Disconnect(ctx)
}

func (d *MongoDB) WriteTx() db.WriteTx {
	return &WriteTx{
		db:         d.db,
		inMem:      make(map[string][]byte),
		collection: d.collection,
	}
}

func (d *MongoDB) Get(key []byte) ([]byte, error) {
	tx := d.WriteTx()
	defer tx.Discard()
	return tx.Get(key)
}

func (d *MongoDB) Iterate(prefix []byte, callback func(key, value []byte) bool) error {
	tx := d.WriteTx()
	defer tx.Discard()
	return tx.Iterate(prefix, callback)
}

func (d *MongoDB) Compact() error {
	// MongoDB handles compaction automatically, so this is a no-op.
	return nil
}

// check that WriteTx implements the db.WriteTx interface
var _ db.WriteTx = (*WriteTx)(nil)

type WriteTx struct {
	db         *mongo.Database
	batch      []mongo.WriteModel
	inMem      map[string][]byte
	collection string
}

func (tx *WriteTx) Get(k []byte) ([]byte, error) {
	if val, ok := tx.inMem[string(k)]; ok {
		return val, nil
	}
	collection := tx.db.Collection(tx.collection)

	filter := bson.M{"_id": string(k)} // Convert to string
	var result KeyVal
	ctx, cancel := context.WithTimeout(context.Background(), MongodbTimeoutQuery)
	defer cancel()
	err := collection.FindOne(ctx, filter).Decode(&result)
	if err == mongo.ErrNoDocuments {
		return nil, db.ErrKeyNotFound
	}
	return result.Value, err
}

func (tx *WriteTx) Iterate(prefix []byte, callback func(key, value []byte) bool) error {
	// First, handle in-memory keys
	for k, v := range tx.inMem {
		if strings.HasPrefix(k, string(prefix)) {
			if !callback([]byte(k), v) {
				return nil
			}
		}
	}

	// Now, handle database keys, but skip those already handled from in-memory map
	collection := tx.db.Collection(tx.collection)

	filter := bson.M{}
	if len(prefix) > 0 {
		filter = bson.M{
			"_id": bson.M{
				"$regex": primitive.Regex{
					Pattern: "^" + string(prefix),
				},
			},
		}
	}
	cursor, err := collection.Find(context.TODO(), filter)
	if err != nil {
		return err
	}
	defer cursor.Close(context.TODO())

	for cursor.Next(context.TODO()) {
		var kv KeyVal
		err := cursor.Decode(&kv)
		if err != nil {
			return err
		}
		if _, exists := tx.inMem[kv.Key]; exists {
			continue
		}
		if !callback([]byte(kv.Key), kv.Value) {
			break
		}
	}
	return nil
}

func (tx *WriteTx) Set(k, v []byte) error {
	if tx.inMem == nil {
		tx.inMem = make(map[string][]byte)
	}
	update := bson.D{
		primitive.E{Key: "$set", Value: bson.D{
			primitive.E{Key: "value", Value: v},
		}},
	}
	model := mongo.NewUpdateOneModel().SetFilter(
		bson.D{
			primitive.E{
				Key:   "_id",
				Value: string(k),
			},
		},
	).SetUpsert(true).SetUpdate(update)

	tx.batch = append(tx.batch, model)
	tx.inMem[string(k)] = v
	return nil
}

func (tx *WriteTx) Delete(k []byte) error {
	model := mongo.NewDeleteOneModel().SetFilter(bson.M{"_id": string(k)}) // Convert to string
	tx.batch = append(tx.batch, model)
	delete(tx.inMem, string(k))
	return nil
}

func (tx *WriteTx) Apply(other db.WriteTx) error {
	var err2 error
	err := other.Iterate(nil, func(k, v []byte) bool {
		err2 = tx.Set(k, v)
		return err2 == nil
	})
	if err2 != nil {
		return err2
	}
	return err
}

func (tx *WriteTx) Commit() error {
	if len(tx.batch) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), MongodbTimeoutCommit)
	defer cancel()
	collection := tx.db.Collection(tx.collection)
	_, err := collection.BulkWrite(ctx, tx.batch)
	return err
}

func (tx *WriteTx) Discard() {
	tx.batch = nil
	tx.inMem = make(map[string][]byte)
}
