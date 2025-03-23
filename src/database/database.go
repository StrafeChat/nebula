package database

import (
	"log"
	"os"
	"time"

	"github.com/gocql/gocql"
	"github.com/redis/go-redis/v9"
	"github.com/scylladb/gocqlx/v3"
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
