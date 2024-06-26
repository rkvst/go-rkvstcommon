// Package secrets gets parameters for Azure services.
// Currently implemented as reading from a JSON file although other methods
// may be introduced.
package secrets

import (
	"encoding/json"
	"os"

	"github.com/datatrails/go-datatrails-common/logger"
)

type Secrets struct {
	Account string `json:"account"`
	URL     string `json:"url"`
	Key     string `json:"key"`
}

// New parses secrets file.
func readFile(secretsFile string) ([]byte, error) {
	logger.Sugar.Infof("Get secrets %s", secretsFile)
	data, err := os.ReadFile(secretsFile)
	if err != nil {
		logger.Sugar.Infof(
			"unable to read secrets file %s: %v",
			secretsFile,
			err,
		)
		return nil, err
	}
	return data, err
}

func New(secretsFile string) (*Secrets, error) {
	data, err := readFile(secretsFile)
	if err != nil {
		return nil, err
	}

	secrets := &Secrets{}
	err = json.Unmarshal(data, &secrets)
	if err != nil {
		logger.Sugar.Infof(
			"unable to marshal json in secrets file %s: %v",
			secretsFile,
			err,
		)
		return nil, err
	}
	logger.Sugar.Debugf(
		"JSON secrets account %s url %s",
		secrets.Account,
		secrets.URL,
	)

	return secrets, nil
}
