package database

import (
	"log"
	"os"
	"time"

	"github.com/gocql/gocql"
	"github.com/redis/go-redis/v9"
	"github.com/scylladb/gocqlx/v3"

	"github.com/StrafeChat/nebula/src/database/models"
)

var (
	Session *gocqlx.Session
	Rdb     *redis.Client
)

func InitDB() error {
	/*_ Connect to ScyllaDB _*/
	cluster := gocql.NewCluster(os.Getenv("SCYLLA_HOST"))
	cluster.Keyspace = os.Getenv("SCYLLA_KEYSPACE")
	cluster.Consistency = gocql.Quorum
	cluster.Timeout = time.Second * 5

	session, err := gocqlx.WrapSession(cluster.CreateSession())
	if err != nil {
		log.Fatal(err)
	}
	Session = &session
	log.Println("Connected to ScyllaDB.")

	if err := CreateSchema(); err != nil {
		log.Fatalf("Failed to create schema: %v", err)
		return err
	}

	return nil
}

func InitRedis() error {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "127.0.0.1:6379"
	}

	Rdb = redis.NewClient(&redis.Options{
		Addr: redisHost,
	})

	log.Println("Connected to Redis.")
	return nil
}

/*_ Create all Tables and types _*/
func CreateSchema() error {
	log.Println("Creating database schema...")
	models := []interface{}{
		&models.File{},
	}

	for _, model := range models {
		if schemaModel, ok := model.(interface{ SchemaDefinition() []string }); ok {
			for _, stmt := range schemaModel.SchemaDefinition() {
				log.Printf("Executing schema statement: %s", stmt)
				if err := Session.ExecStmt(stmt); err != nil {
					log.Printf("Schema creation error: %v", err)
					return err
				}
			}
		}
	}

	log.Println("Database schema created successfully")
	return nil
}
