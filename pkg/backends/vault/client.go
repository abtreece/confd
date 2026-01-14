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

// getRequiredParameter retrieves a required parameter from the configuration.
// Returns an error if the parameter is missing or empty.
func getRequiredParameter(key string, parameters map[string]string) (string, error) {
	value := parameters[key]
	if value == "" {
		return "", fmt.Errorf("required parameter %q is missing from configuration", key)
	}
	return value, nil
}

// authenticate with the remote client
func authenticate(c *vaultapi.Client, authType string, params map[string]string) error {
	var secret *vaultapi.Secret
	var err error

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
		roleID, err := getRequiredParameter("role-id", params)
		if err != nil {
			return err
		}
		secretID, err := getRequiredParameter("secret-id", params)
		if err != nil {
			return err
		}
		secret, err = c.Logical().Write(url, map[string]interface{}{
			"role_id":   roleID,
			"secret_id": secretID,
		})
		if err != nil {
			return err
		}
	case "app-id":
		appID, err := getRequiredParameter("app-id", params)
		if err != nil {
			return err
		}
		userID, err := getRequiredParameter("user-id", params)
		if err != nil {
			return err
		}
		secret, err = c.Logical().Write(url, map[string]interface{}{
			"app_id":  appID,
			"user_id": userID,
		})
		if err != nil {
			return err
		}
	case "github":
		token, err := getRequiredParameter("token", params)
		if err != nil {
			return err
		}
		secret, err = c.Logical().Write(url, map[string]interface{}{
			"token": token,
		})
		if err != nil {
			return err
		}
	case "token":
		token, err := getRequiredParameter("token", params)
		if err != nil {
			return err
		}
		c.SetToken(token)
		secret, err = c.Logical().Read("/auth/token/lookup-self")
		if err != nil {
			return err
		}
	case "userpass":
		username, err := getRequiredParameter("username", params)
		if err != nil {
			return err
		}
		password, err := getRequiredParameter("password", params)
		if err != nil {
			return err
		}
		secret, err = c.Logical().Write(fmt.Sprintf("%s/%s", url, username), map[string]interface{}{
			"password": password,
		})
		if err != nil {
			return err
		}
	case "kubernetes":
		jwt, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
		if err != nil {
			return fmt.Errorf("failed to read kubernetes service account token: %w", err)
		}
		roleID, err := getRequiredParameter("role-id", params)
		if err != nil {
			return err
		}
		secret, err = c.Logical().Write(url, map[string]interface{}{
			"jwt":  string(jwt),
			"role": roleID,
		})
		if err != nil {
			return err
		}
	case "cert":
		secret, err = c.Logical().Write(url, map[string]interface{}{})
		if err != nil {
			return err
		}
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
		if err != nil {
			log.Error("failed to get mount info for %s: %v", mount, err)
			continue
		}
		if resp == nil || resp.Body == nil {
			log.Error("empty response getting mount info for %s", mount)
			continue
		}

		secret, err := vaultapi.ParseSecret(resp.Body)
		resp.Body.Close() // Close immediately after parsing to avoid resource leak in loop
		if err != nil {
			log.Error("failed to parse secret for %s: %v", mount, err)
			continue
		}
		if secret == nil || secret.Data == nil {
			log.Error("empty secret data for %s", mount)
			continue
		}

		engine := secret.Data["type"]

		if engine == "kv" {
			version, err := getKVVersion(secret.Data)
			if err != nil {
				log.Error("failed to get KV version for %s: %v", mount, err)
				continue
			}
			var key string
			secrets := recursiveListSecretWithLogical(c.logical, mount, key, version)
			switch version {
			case "", "1":
				for _, secretPath := range secrets {
					secretResp, err := c.logical.Read(secretPath)
					if err != nil {
						log.Warning("failed to read secret %s: %v", secretPath, err)
						continue
					}
					if secretResp == nil || secretResp.Data == nil {
						continue
					}
					js, err := json.Marshal(secretResp.Data)
					if err != nil {
						log.Warning("failed to marshal secret %s: %v", secretPath, err)
						continue
					}
					vars[secretPath] = string(js)
					flatten(secretPath, secretResp.Data, mount, vars)
				}
			case "2":
				for _, secretPath := range secrets {
					secretResp, err := c.logical.Read(secretPath)
					if err != nil {
						log.Warning("failed to read secret %s: %v", secretPath, err)
						continue
					}
					if secretResp == nil || secretResp.Data == nil {
						continue
					}
					data := secretResp.Data["data"]
					js, err := json.Marshal(data)
					if err != nil {
						log.Warning("failed to marshal secret %s: %v", secretPath, err)
						continue
					}
					vars[secretPath] = string(js)
					flatten(secretPath, data, mount, vars)
				}
			}
		} else {
			log.Error("Engine type %s is not supported", engine)
		}
	}
	return vars, nil
}

// getKVVersion safely extracts the KV version from secret data
func getKVVersion(data map[string]interface{}) (string, error) {
	options, ok := data["options"].(map[string]interface{})
	if !ok {
		// Default to version 1 if options not present
		return "1", nil
	}
	version, ok := options["version"].(string)
	if !ok {
		return "1", nil
	}
	return version, nil
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

// buildListPath returns the correct path for listing secrets based on KV version.
// Note: key parameter may contain a leading slash (e.g., "/mykey") which will
// result in paths like "/secret/metadata//mykey" - this is expected behavior.
func buildListPath(basePath, key, version string) string {
	switch version {
	case "2":
		return basePath + "/metadata/" + key
	default: // "", "1", or any other value defaults to v1
		return basePath + key
	}
}

// buildSecretPath returns the correct path for reading secrets based on KV version.
// Note: key parameter may contain a leading slash (e.g., "/mykey") which will
// result in paths like "/secret/data//mykey" - this is expected behavior.
func buildSecretPath(basePath, key, version string) string {
	switch version {
	case "2":
		return basePath + "/data" + key
	default: // "", "1", or any other value defaults to v1
		return basePath + key
	}
}

// listSecretWithLogical returns a list of secrets from Vault using the vaultLogical interface
func listSecretWithLogical(logical vaultLogical, path string, key string, version string) (*vaultapi.Secret, error) {
	listPath := buildListPath(path, key, version)
	secret, err := logical.List(listPath)
	if err != nil {
		log.Warning("Couldn't list from the Vault: %v", err)
	}
	return secret, err
}

// recursiveListSecretWithLogical returns a list of secrets paths from Vault using the vaultLogical interface
func recursiveListSecretWithLogical(logical vaultLogical, basePath string, key string, version string) []string {
	var results []string
	secretList, err := listSecretWithLogical(logical, basePath, key, version)
	if err != nil || secretList == nil || secretList.Data == nil {
		return results
	}

	keys, ok := secretList.Data["keys"].([]interface{})
	if !ok {
		return results
	}

	for _, secret := range keys {
		secretStr, ok := secret.(string)
		if !ok {
			continue
		}
		if strings.HasSuffix(secretStr, "/") {
			// It's a directory, recurse
			newKey := key + "/" + strings.TrimSuffix(secretStr, "/")
			subResults := recursiveListSecretWithLogical(logical, basePath, newKey, version)
			results = append(results, subResults...)
		} else {
			// It's a secret
			newKey := key + "/" + secretStr
			secretPath := buildSecretPath(basePath, newKey, version)
			results = append(results, secretPath)
		}
	}
	return results
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
