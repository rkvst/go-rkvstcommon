package azkeys

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/rkvst/go-rkvstcommon/logger"
)

const (
	aADEndpointAddress = "http://169.254.169.254/"
	oauthPathTokenPath = "metadata/identity/oauth2/token"
	aPIVersion         = "2018-02-01"

	connectionTimeout = 30 * time.Second
)

type aADProvider struct {
	resourceName string
}

func (a *aADProvider) OAuthToken() string {
	var err error
	client := &http.Client{
		Timeout: connectionTimeout,
	}

	query := url.Values{}
	query.Add("api-version", aPIVersion)
	query.Add("resource", a.resourceName)
	url := aADEndpointAddress + oauthPathTokenPath + "?" + query.Encode()
	logger.Sugar.Debugf("get token from: %s", url)

	var reader io.ReadCloser
	reader, err = doReq(client, url)
	if err != nil {
		logger.Sugar.Infof("could not get token - will try again")
		return ""
	}

	defer reader.Close()

	type tokenResp struct {
		AccessToken string `json:"access_token"`
	}

	token := tokenResp{}
	err = json.NewDecoder(reader).Decode(&token)
	if err != nil {
		logger.Sugar.Infof("failed to unmarshal token: %v", err)
		return ""
	}

	return token.AccessToken
}

func doReq(client *http.Client, url string) (io.ReadCloser, error) {
	resp, err := client.Get(url)

	if err != nil {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		logger.Sugar.Infof("failed to get token: %v", err)
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		if resp.Body != nil {
			resp.Body.Close()
		}
		logger.Sugar.Infof("bad response status: %v", resp.Status)
		return nil, errors.New("bad response status")
	}

	return resp.Body, nil
}
