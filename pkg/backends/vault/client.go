package vault

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/abtreece/confd/pkg/log"
	vaultapi "github.com/hashicorp/vault/api"
)

// vaultLogical defines the interface for Vault logical operations.
// This allows for mocking in tests.
type vaultLogical interface {
	List(path string) (*vaultapi.Secret, error)
	Read(path string) (*vaultapi.Secret, error)
	ReadRaw(path string) (*vaultapi.Response, error)
	Write(path string, data map[string]interface{}) (*vaultapi.Secret, error)
}

// Client is a wrapper around the vault client
type Client struct {
	client  *vaultapi.Client
	logical vaultLogical
}

// get a
func getParameter(key string, parameters map[string]string) string {
	value := parameters[key]
	if value == "" {
		// panic if a configuration is missing
		panic(fmt.Sprintf("%s is missing from configuration", key))
	}
	return value
}

// panicToError converts a panic to an error
func panicToError(err *error) {
	if r := recover(); r != nil {
		switch t := r.(type) {
		case string:
			*err = errors.New(t)
		case error:
			*err = t
		default: // panic again if we don't know how to handle
			panic(r)
		}
	}
}

// authenticate with the remote client
func authenticate(c *vaultapi.Client, authType string, params map[string]string) (err error) {
	var secret *vaultapi.Secret

	// handle panics gracefully by creating an error
	// this would happen when we get a parameter that is missing
	defer panicToError(&err)

	path := params["path"]
	if path == "" {
		path = authType
		if authType == "app-role" {
			path = "approle"
		}
	}
	url := fmt.Sprintf("/auth/%s/login", path)

	switch authType {
	case "app-role":
		secret, err = c.Logical().Write(url, map[string]interface{}{
			"role_id":   getParameter("role-id", params),
			"secret_id": getParameter("secret-id", params),
		})
	case "app-id":
		secret, err = c.Logical().Write(url, map[string]interface{}{
			"app_id":  getParameter("app-id", params),
			"user_id": getParameter("user-id", params),
		})
	case "github":
		secret, err = c.Logical().Write(url, map[string]interface{}{
			"token": getParameter("token", params),
		})
	case "token":
		c.SetToken(getParameter("token", params))
		secret, err = c.Logical().Read("/auth/token/lookup-self")
	case "userpass":
		username, password := getParameter("username", params), getParameter("password", params)
		secret, err = c.Logical().Write(fmt.Sprintf("%s/%s", url, username), map[string]interface{}{
			"password": password,
		})
	case "kubernetes":
		jwt, jwtErr := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
		if jwtErr != nil {
			return jwtErr
		}
		secret, err = c.Logical().Write(url, map[string]interface{}{
			"jwt":  string(jwt[:]),
			"role": getParameter("role-id", params),
		})
	case "cert":
		secret, err = c.Logical().Write(url, map[string]interface{}{})
	}

	if err != nil {
		return err
	}

	// if the token has already been set
	if c.Token() != "" {
		return nil
	}

	if secret == nil || secret.Auth == nil {
		return errors.New("unable to authenticate")
	}

	log.Debug("client authenticated with auth backend: %s", authType)
	// the default place for a token is in the auth section
	// otherwise, the backend will set the token itself
	c.SetToken(secret.Auth.ClientToken)
	return nil
}

func getConfig(address, cert, key, caCert string) (*vaultapi.Config, error) {
	conf := vaultapi.DefaultConfig()
	conf.Address = address

	tlsConfig := &tls.Config{}
	if cert != "" && key != "" {
		clientCert, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{clientCert}
	}

	if caCert != "" {
		ca, err := os.ReadFile(caCert)
		if err != nil {
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(ca)
		tlsConfig.RootCAs = caCertPool
	}

	conf.HttpClient.Transport = &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return conf, nil
}

// New returns an *vault.Client with a connection to named machines.
// It returns an error if a connection to the cluster cannot be made.
func New(address, authType string, params map[string]string) (*Client, error) {
	if authType == "" {
		return nil, errors.New("you have to set the auth type when using the vault backend")
	}
	log.Info("Vault authentication backend set to %s", authType)
	conf, err := getConfig(address, params["cert"], params["key"], params["caCert"])

	if err != nil {
		return nil, err
	}

	c, err := vaultapi.NewClient(conf)
	if err != nil {
		return nil, err
	}

	if err := authenticate(c, authType, params); err != nil {
		return nil, err
	}
	return &Client{client: c, logical: c.Logical()}, nil
}

// GetValues queries Vault for keys prefixed by prefix.
func (c *Client) GetValues(ctx context.Context, paths []string) (map[string]string, error) {
	vars := make(map[string]string)
	var mounts []string
	for _, path := range paths {
		path = strings.TrimRight(path, "/*")
		mounts = append(mounts, getMount(path))
	}
	mounts = uniqMounts(mounts)

	for _, mount := range mounts {
		resp, err := c.logical.ReadRaw("/sys/internal/ui/mounts/" + mount)
		if resp != nil {
			defer resp.Body.Close()
		}
		if err != nil {
			fmt.Printf("there was an error getting %s version", mount)
			fmt.Println(err)
		}

		secret, err := vaultapi.ParseSecret(resp.Body)
		if err != nil {
			fmt.Printf("there was an error parsing secrets of %s", mount)
			fmt.Println(err)
		}

		engine := secret.Data["type"]

		if engine == "kv" {
			options := secret.Data["options"]
			versionRaw := options.(map[string]interface{})["version"]
			version := versionRaw.(string)
			var key string
			switch version {
			case "", "1":
				for _, secret := range recursiveListSecretWithLogical(c.logical, mount, key, version) {
					resp, _ := c.logical.Read(secret)

					js, _ := json.Marshal(resp.Data)
					vars[secret] = string(js)
					flatten(secret, resp.Data, mount, vars)
				}
			case "2":
				for _, secret := range recursiveListSecretWithLogical(c.logical, mount, key, version) {
					resp, _ := c.logical.Read(secret)

					js, _ := json.Marshal(resp.Data["data"])
					vars[secret] = string(js)
					flatten(secret, resp.Data["data"], mount, vars)
				}
			}
		} else {
			log.Error("Engine type %s is not supported", engine)
		}
	}
	return vars, nil
}

// recursively walks on all the keys of a specific secret and set them in the variables map
func flatten(key string, value interface{}, mount string, vars map[string]string) {
	switch value.(type) {
	case string:
		key = strings.ReplaceAll(key, "data/", "")
		vars[key] = value.(string)
	case map[string]interface{}:
		inner := value.(map[string]interface{})
		for innerKey, innerValue := range inner {
			innerKey = path.Join(key, "/", innerKey)
			flatten(innerKey, innerValue, mount, vars)
		}
	default: // we don't know how to handle non string or maps of strings
		log.Warning("type of '%s' is not supported (%T)", key, value)
	}
}

var secretListPath []string

// listSecretWithLogical returns a list of secrets from Vault using the vaultLogical interface
func listSecretWithLogical(logical vaultLogical, path string, key string, version string) (*vaultapi.Secret, error) {
	switch version {
	case "", "1":
		secret, err := logical.List(path + key)
		if err != nil {
			log.Warning("Couldn't list from the Vault.")
		}
		return secret, err
	case "2":
		secret, err := logical.List(path + "/metadata/" + key)
		if err != nil {
			log.Warning("Couldn't list from the Vault.")
		}
		return secret, err
	}
	return nil, nil
}

// recursiveListSecretWithLogical returns a list of secrets paths from Vault using the vaultLogical interface
func recursiveListSecretWithLogical(logical vaultLogical, path string, key string, version string) []string {
	switch version {
	case "", "1":
		secretList, err := listSecretWithLogical(logical, path, key, version)
		if err == nil && secretList != nil {
			for _, secret := range secretList.Data["keys"].([]interface{}) {
				if strings.HasSuffix(secret.(string), "/") {
					key := key + "/" + strings.TrimSuffix(secret.(string), "/")
					recursiveListSecretWithLogical(logical, path, key, version)
				} else {
					key := key + "/" + strings.TrimSuffix(secret.(string), "/")
					secretListPath = append([]string{path + key}, secretListPath...)
				}
			}
		} else {
			secretListPath = append([]string{path}, secretListPath...)
		}
		return secretListPath
	case "2":
		secretList, err := listSecretWithLogical(logical, path, key, version)
		if err == nil && secretList != nil {
			for _, secret := range secretList.Data["keys"].([]interface{}) {
				if strings.HasSuffix(secret.(string), "/") {
					key := key + "/" + strings.TrimSuffix(secret.(string), "/")
					recursiveListSecretWithLogical(logical, path, key, version)
				} else {
					key := key + "/" + strings.TrimSuffix(secret.(string), "/")
					secretListPath = append([]string{path + "/data" + key}, secretListPath...)
				}
			}
		} else {
			secretListPath = append([]string{path + "data/"}, secretListPath...)
		}
		return secretListPath
	}
	return nil
}

// ListSecret returns a list of secrets from Vault
func ListSecret(vault *vaultapi.Client, path string, key string, version string) (*vaultapi.Secret, error) {
	switch version {
	case "1":
		secret, err := vault.Logical().List(path + key)
		if err != nil {
			log.Warning("Couldn't list from the Vault.")
		}
		return secret, err
	case "2":
		secret, err := vault.Logical().List(path + "/metadata/" + key)
		if err != nil {
			log.Warning("Couldn't list from the Vault.")
		}
		return secret, err
	}
	return nil, nil
}

// RecursiveListSecret returns a list of secrets paths from Vault
func RecursiveListSecret(vault *vaultapi.Client, path string, key string, version string) []string {
	switch version {
	case "1":
		secretList, err := ListSecret(vault, path, key, version)
		if err == nil && secretList != nil {
			for _, secret := range secretList.Data["keys"].([]interface{}) {
				if strings.HasSuffix(secret.(string), "/") {
					key := key + "/" + strings.TrimSuffix(secret.(string), "/")
					RecursiveListSecret(vault, path, key, version)
				} else {
					key := key + "/" + strings.TrimSuffix(secret.(string), "/")
					secretListPath = append([]string{path + key}, secretListPath...)
				}
			}
		} else {
			secretListPath = append([]string{path}, secretListPath...)
		}
		return secretListPath
	case "2":
		secretList, err := ListSecret(vault, path, key, version)
		if err == nil && secretList != nil {
			for _, secret := range secretList.Data["keys"].([]interface{}) {
				if strings.HasSuffix(secret.(string), "/") {
					key := key + "/" + strings.TrimSuffix(secret.(string), "/")
					RecursiveListSecret(vault, path, key, version)
				} else {
					key := key + "/" + strings.TrimSuffix(secret.(string), "/")
					secretListPath = append([]string{path + "/data" + key}, secretListPath...)
				}
			}
		} else {
			secretListPath = append([]string{path + "data/"}, secretListPath...)
		}
		return secretListPath
	}
	return nil
}

func getMount(path string) string {
	split := strings.Split(path, string(os.PathSeparator))
	return "/" + split[1]
}

func uniqMounts(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}

// WatchPrefix - not implemented at the moment
func (c *Client) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	<-stopChan
	return 0, nil
}

// HealthCheck verifies the backend connection is healthy.
// It checks the Vault server health status.
func (c *Client) HealthCheck(ctx context.Context) error {
	_, err := c.client.Sys().Health()
	return err
}
