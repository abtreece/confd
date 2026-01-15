package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	zk "github.com/go-zookeeper/zk"
)

func fatal(context string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s: %v\n", context, err)
		os.Exit(1)
	}
}

func zkWrite(key string, value string, conn *zk.Conn) error {
	exists, stat, err := conn.Exists(key)
	if err != nil {
		return fmt.Errorf("checking existence of %s: %w", key, err)
	}

	if exists {
		_, err = conn.Set(key, []byte(value), stat.Version)
		if err != nil {
			return fmt.Errorf("updating %s: %w", key, err)
		}
	} else {
		_, err = conn.Create(key, []byte(value), int32(0), zk.WorldACL(zk.PermAll))
		if err != nil {
			return fmt.Errorf("creating %s: %w", key, err)
		}
	}
	return nil
}

func parseJSON(prefix string, data interface{}, conn *zk.Conn) error {
	switch t := data.(type) {
	case map[string]interface{}:
		for k, v := range t {
			if prefix != "" {
				if err := zkWrite(prefix, "", conn); err != nil {
					return err
				}
			}
			if err := parseJSON(prefix+"/"+k, v, conn); err != nil {
				return err
			}
		}
	case []interface{}:
		for i, v := range t {
			if err := parseJSON(prefix+"["+strconv.Itoa(i)+"]", v, conn); err != nil {
				return err
			}
		}
	case string:
		if err := zkWrite(prefix, t, conn); err != nil {
			return err
		}
		fmt.Printf("%s = %q\n", prefix, t)
	default:
		fmt.Printf("Unhandled type: %T\n", t)
	}
	return nil
}

func main() {
	zkNode := os.Getenv("ZOOKEEPER_NODE")
	if zkNode == "" {
		fmt.Fprintln(os.Stderr, "ERROR: ZOOKEEPER_NODE environment variable not set")
		os.Exit(1)
	}

	// Read and parse the test data JSON
	data, err := os.ReadFile("test.json")
	fatal("reading test.json", err)

	var jsonData interface{}
	err = json.Unmarshal(data, &jsonData)
	fatal("parsing JSON", err)

	// Connect to Zookeeper (node should be hostname, port is added separately)
	conn, _, err := zk.Connect([]string{zkNode}, time.Second*10)
	fatal("connecting to Zookeeper", err)
	defer conn.Close()

	// Load the test data into Zookeeper
	err = parseJSON("", jsonData, conn)
	fatal("loading data into Zookeeper", err)

	fmt.Println("Zookeeper data loaded successfully")
}
