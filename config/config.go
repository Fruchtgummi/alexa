package config

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Product      string    `json:"product_id"`
	ClientId     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func LoadConfig() (*Config, error) {
	path := filepath.Join(os.Getenv("HOME"), ".alexa.json")

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}

	defer f.Close()

	var cfg Config

	err = json.NewDecoder(f).Decode(&cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func WriteConfig(cfg *Config) error {
	path := filepath.Join(os.Getenv("HOME"), ".alexa.json")

	f, err := os.Create(path)
	if err != nil {
		return err
	}

	defer f.Close()

	return json.NewEncoder(f).Encode(&cfg)
}

func GetToken() (string, error) {
	config, err := LoadConfig()
	if err != nil {
		return "", err
	}

	if config.ExpiresAt.After(time.Now()) {
		return config.AccessToken, nil
	}

	form := url.Values{}

	form.Add("client_id", config.ClientId)
	form.Add("client_secret", config.ClientSecret)
	form.Add("refresh_token", config.RefreshToken)
	form.Add("grant_type", "refresh_token")

	req, err := http.NewRequest("POST", "https://api.amazon.com/auth/o2/token", strings.NewReader(form.Encode()))
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()

	var oauthResponse struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}

	err = json.NewDecoder(resp.Body).Decode(&oauthResponse)
	if err != nil {
		log.Fatal(err)
	}

	config.AccessToken = oauthResponse.AccessToken
	config.ExpiresAt = time.Now().UTC().Add(time.Duration(oauthResponse.ExpiresIn) * time.Second)

	err = WriteConfig(config)
	if err != nil {
		return "", err
	}

	return oauthResponse.AccessToken, nil
}
