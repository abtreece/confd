package zookeeper

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/abtreece/confd/pkg/log"
	zk "github.com/go-zookeeper/zk"
)

// zkConn defines the interface for Zookeeper connection operations.
// This allows for mocking in tests.
type zkConn interface {
	Children(path string) ([]string, *zk.Stat, error)
	Get(path string) ([]byte, *zk.Stat, error)
	Exists(path string) (bool, *zk.Stat, error)
	GetW(path string) ([]byte, *zk.Stat, <-chan zk.Event, error)
	ChildrenW(path string) ([]string, *zk.Stat, <-chan zk.Event, error)
}

// Client provides a wrapper around the zookeeper client
type Client struct {
	client zkConn
}

func NewZookeeperClient(machines []string) (*Client, error) {
	c, _, err := zk.Connect(machines, time.Second) //*10)
	if err != nil {
		panic(err)
	}
	return &Client{c}, nil
}

func nodeWalk(prefix string, c *Client, vars map[string]string) error {
	var s string
	l, stat, err := c.client.Children(prefix)
	if err != nil {
		return err
	}

	if stat.NumChildren == 0 {
		b, _, err := c.client.Get(prefix)
		if err != nil {
			return err
		}
		vars[prefix] = string(b)

	} else {
		for _, key := range l {
			if prefix == "/" {
				s = "/" + key
			} else {
				s = prefix + "/" + key
			}
			_, stat, err := c.client.Exists(s)
			if err != nil {
				return err
			}
			if stat.NumChildren == 0 {
				b, _, err := c.client.Get(s)
				if err != nil {
					return err
				}
				vars[s] = string(b)
			} else {
				nodeWalk(s, c, vars)
			}
		}
	}
	return nil
}

func (c *Client) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	vars := make(map[string]string)
	for _, v := range keys {
		v = strings.Replace(v, "/*", "", -1)
		_, _, err := c.client.Exists(v)
		if err != nil {
			return vars, err
		}
		err = nodeWalk(v, c, vars)
		if err != nil {
			return vars, err
		}
	}
	return vars, nil
}

type watchResponse struct {
	waitIndex uint64
	err       error
}

func (c *Client) watch(key string, respChan chan watchResponse, cancelRoutine chan bool) {
	_, _, keyEventCh, err := c.client.GetW(key)
	if err != nil {
		respChan <- watchResponse{0, err}
	}
	_, _, childEventCh, err := c.client.ChildrenW(key)
	if err != nil {
		respChan <- watchResponse{0, err}
	}

	for {
		select {
		case e := <-keyEventCh:
			if e.Type == zk.EventNodeDataChanged {
				respChan <- watchResponse{1, e.Err}
			}
		case e := <-childEventCh:
			if e.Type == zk.EventNodeChildrenChanged {
				respChan <- watchResponse{1, e.Err}
			}
		case <-cancelRoutine:
			log.Debug("Stop watching: %s", key)
			// There is no way to stop GetW/ChildrenW so just quit
			return
		}
	}
}

func (c *Client) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	// return something > 0 to trigger a key retrieval from the store
	if waitIndex == 0 {
		return 1, nil
	}

	// List the childrens first
	entries, err := c.GetValues(ctx, []string{prefix})
	if err != nil {
		return 0, err
	}

	respChan := make(chan watchResponse)
	cancelRoutine := make(chan bool)
	defer close(cancelRoutine)

	//watch all subfolders for changes
	watchMap := make(map[string]string)
	for k, _ := range entries {
		for _, v := range keys {
			if strings.HasPrefix(k, v) {
				for dir := filepath.Dir(k); dir != "/"; dir = filepath.Dir(dir) {
					if _, ok := watchMap[dir]; !ok {
						watchMap[dir] = ""
						log.Debug("Watching: %s", dir)
						go c.watch(dir, respChan, cancelRoutine)
					}
				}
				break
			}
		}
	}

	//watch all keys in prefix for changes
	for k, _ := range entries {
		for _, v := range keys {
			if strings.HasPrefix(k, v) {
				log.Debug("Watching: %s", k)
				go c.watch(k, respChan, cancelRoutine)
				break
			}
		}
	}

	for {
		select {
		case <-stopChan:
			return waitIndex, nil
		case r := <-respChan:
			return r.waitIndex, r.err
		}
	}
}
