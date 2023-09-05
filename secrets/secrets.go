// Package secrets gets parameters for Azure services.
// Currently implemented as reading from a JSON file although other methods
// may be introduced.
package secrets

import (
	"encoding/json"
	"os"

	"go.uber.org/zap"

	"github.com/rkvst/go-rkvstcommon/logger"
)

type Secrets struct {
	Account string `json:"account"`
	URL     string `json:"url"`
	Key     string `json:"key"`
}

// New parses secrets file.
func readFile(secretsFile string) ([]byte, error) {
	logger.Plain.Info(
		"Get secrets",
		zap.String("secretsFile", secretsFile),
	)
	data, err := os.ReadFile(secretsFile)
	if err != nil {
		logger.Plain.Info(
			"unable to read secrets file",
			zap.String("secretsFile", secretsFile),
			zap.Error(err),
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
		logger.Plain.Info(
			"unable to marshal json in secrets file",
			zap.String("secretsFile", secretsFile),
			zap.Error(err),
		)
		return nil, err
	}
	logger.Plain.Debug(
		"JSON secrets",
		zap.String("account", secrets.Account),
		zap.String("url", secrets.URL),
	)

	return secrets, nil
}
