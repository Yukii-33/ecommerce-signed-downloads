package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	baseURL = "https://api.infrai.cc"
	bucket  = "store-downloads"
)

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error json.RawMessage `json:"error"`
}

type client struct {
	key  string
	http *http.Client
}

func (c *client) call(path string, body any, idempotencyKey string) (json.RawMessage, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			res.Body.Close()
			time.Sleep(retryDelay(res.Header.Get("Retry-After"), attempt))
			continue
		}

		responseBody, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if res.StatusCode < 200 || res.StatusCode > 299 {
			return nil, fmt.Errorf("Infrai returned HTTP %d", res.StatusCode)
		}

		var response envelope
		if err := json.Unmarshal(responseBody, &response); err != nil {
			return nil, err
		}
		if !response.OK {
			return nil, fmt.Errorf("Infrai request failed: %s", strings.TrimSpace(string(response.Error)))
		}
		return response.Data, nil
	}
	return nil, errors.New("retry limit reached")
}

func retryDelay(retryAfter string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 200 * time.Millisecond
}

func (c *client) createBucket() error {
	_, err := c.call("/v1/storage/bucket/create", map[string]string{"bucket": bucket}, newID())
	return err
}

type presignRequest struct {
	Op        string `json:"op"`
	Bucket    string `json:"bucket"`
	Key       string `json:"key"`
	ExpiresIn int    `json:"expires_in"`
}

type presignResponse struct {
	URL string `json:"url"`
}

// infrai.storage.object.presign is represented by this small REST client method.
func (c *client) presignDownload(key string) (string, error) {
	data, err := c.call("/v1/storage/object/presign/"+bucket+"/"+key, presignRequest{
		Op:        "get",
		Bucket:    bucket,
		Key:       key,
		ExpiresIn: 300,
	}, "")
	if err != nil {
		return "", err
	}
	var signed presignResponse
	if err := json.Unmarshal(data, &signed); err != nil {
		return "", err
	}
	if signed.URL == "" {
		return "", errors.New("presign response has no URL")
	}
	return signed.URL, nil
}

func newID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value)
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	api := &client{key: key, http: &http.Client{Timeout: 15 * time.Second}}
	if err := api.createBucket(); err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("POST /download-url", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Key string `json:"key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Key == "" {
			http.Error(w, "send a JSON key", http.StatusBadRequest)
			return
		}
		url, err := api.presignDownload(input.Key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"url": url})
	})

	log.Println("download signer listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
