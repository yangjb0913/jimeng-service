package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jimeng-service/config"
	"jimeng-service/models"

	"github.com/google/uuid"
)

type JimengClient struct {
	cfg       *config.JimengConfig
	keyPool   *KeyPool
	httpClient *http.Client
}

type SubmitTaskRequest struct {
	ReqKey       string   `json:"req_key"`
	Prompt       string   `json:"prompt,omitempty"`
	ImageURLs    []string `json:"image_urls,omitempty"`
	BinaryDataBase64 []string `json:"binary_data_base64,omitempty"`
	Seed         int      `json:"seed,omitempty"`
	Frames       int      `json:"frames,omitempty"`
	AspectRatio  string   `json:"aspect_ratio,omitempty"`
	Width        int      `json:"width,omitempty"`
	Height       int      `json:"height,omitempty"`
	Scale        float64  `json:"scale,omitempty"`
	ForceSingle  bool     `json:"force_single,omitempty"`
	TemplateID   string   `json:"template_id,omitempty"`
	CameraStrength string `json:"camera_strength,omitempty"`
}

type SubmitTaskResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Data      struct {
		TaskID string `json:"task_id"`
	} `json:"data"`
}

type QueryTaskResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Data      struct {
		Status            string   `json:"status"`
		VideoURL          string   `json:"video_url"`
		ImageURLs         []string `json:"image_urls"`
		BinaryDataBase64  []string `json:"binary_data_base64"`
		AIGCMetaTagged   bool     `json:"aigc_meta_tagged"`
	} `json:"data"`
}

func NewJimengClient(cfg *config.JimengConfig, keyPool *KeyPool) *JimengClient {
	return &JimengClient{
		cfg:       cfg,
		keyPool:   keyPool,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *JimengClient) SubmitTask(function, prompt string, imageURLs []string, extra map[string]interface{}) (string, *models.APIKey, error) {
	reqKey := models.FunctionReqKeys[models.FunctionType(function)]
	if reqKey == "" {
		return "", nil, fmt.Errorf("unknown function: %s", function)
	}

	apiKey, err := c.keyPool.SelectKey(function)
	if err != nil {
		return "", nil, err
	}

	reqBody := SubmitTaskRequest{
		ReqKey: reqKey,
		Prompt: prompt,
	}

	if len(imageURLs) > 0 {
		localURLs, remoteURLs := filterLocalURLs(imageURLs)

		supportsBase64 := functionSupportsBase64(function)
		if supportsBase64 && len(localURLs) > 0 {
			base64Data, err := convertLocalImagesToBase64(localURLs)
			if err != nil {
				log.Printf("Failed to convert local images to base64: %v", err)
			} else {
				reqBody.BinaryDataBase64 = base64Data
			}
		} else if len(localURLs) > 0 {
			reqBody.ImageURLs = append(reqBody.ImageURLs, localURLs...)
		}

		if len(remoteURLs) > 0 {
			reqBody.ImageURLs = append(reqBody.ImageURLs, remoteURLs...)
		}
	}

	if frames, ok := extra["frames"].(int); ok {
		reqBody.Frames = frames
	}
	if aspectRatio, ok := extra["aspect_ratio"].(string); ok {
		reqBody.AspectRatio = aspectRatio
	}
	if width, ok := extra["width"].(int); ok {
		reqBody.Width = width
	}
	if height, ok := extra["height"].(int); ok {
		reqBody.Height = height
	}
	if scale, ok := extra["scale"].(float64); ok {
		reqBody.Scale = scale
	}
	if forceSingle, ok := extra["force_single"].(bool); ok {
		reqBody.ForceSingle = forceSingle
	}
	if templateID, ok := extra["template_id"].(string); ok {
		reqBody.TemplateID = templateID
	}
	if cameraStrength, ok := extra["camera_strength"].(string); ok {
		reqBody.CameraStrength = cameraStrength
	}
	if seed, ok := extra["seed"].(int); ok {
		reqBody.Seed = seed
	} else {
		reqBody.Seed = -1
	}

	resp, err := c.doRequest("CVSync2AsyncSubmitTask", apiKey, reqBody)
	if err != nil {
		c.keyPool.MarkFunctionFailed(apiKey.ID, function)
		return "", nil, err
	}

	if resp.Code != 10000 {
		return "", nil, fmt.Errorf("submit failed: %s (code: %d)", resp.Message, resp.Code)
	}

	return resp.Data.TaskID, apiKey, nil
}

func (c *JimengClient) QueryTask(taskID, function string, apiKey *models.APIKey) (*QueryTaskResponse, error) {
	reqKey := models.FunctionReqKeys[models.FunctionType(function)]
	if reqKey == "" {
		return nil, fmt.Errorf("unknown function: %s", function)
	}

	reqBody := map[string]string{
		"req_key": reqKey,
		"task_id": taskID,
	}

	resp := &QueryTaskResponse{}
	err := c.doRequestRaw("CVSync2AsyncGetResult", apiKey, reqBody, resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != 10000 {
		return nil, fmt.Errorf("query failed: %s (code: %d)", resp.Message, resp.Code)
	}

	return resp, nil
}

func (c *JimengClient) doRequest(action string, apiKey *models.APIKey, body interface{}) (*SubmitTaskResponse, error) {
	queryParams := url.Values{}
	queryParams.Set("Action", action)
	queryParams.Set("Version", c.cfg.Version)

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	urlStr := fmt.Sprintf("%s?%s", c.cfg.APIHost, queryParams.Encode())

	log.Printf("[SUBMIT] URL: %s", urlStr)
	log.Printf("[SUBMIT] Body: %s", string(bodyBytes))

	signer := NewSigner(apiKey.AK, apiKey.SK, c.cfg.Region, c.cfg.Service)
	headers := signer.SignRequest("POST", urlStr, bodyBytes)

	log.Printf("[SUBMIT] Headers: %+v", headers)

	req, err := http.NewRequest("POST", urlStr, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	log.Printf("[SUBMIT] Response Status: %d", resp.StatusCode)
	log.Printf("[SUBMIT] Response Body: %s", string(respBody))

	var result SubmitTaskResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v, body: %s", err, string(respBody))
	}

	return &result, nil
}

func (c *JimengClient) doRequestRaw(action string, apiKey *models.APIKey, body map[string]string, result interface{}) error {
	queryParams := url.Values{}
	queryParams.Set("Action", action)
	queryParams.Set("Version", c.cfg.Version)

	bodyBytes, _ := json.Marshal(body)

	urlStr := fmt.Sprintf("%s?%s", c.cfg.APIHost, queryParams.Encode())

	log.Printf("[QUERY] URL: %s", urlStr)
	log.Printf("[QUERY] Body: %s", string(bodyBytes))

	signer := NewSigner(apiKey.AK, apiKey.SK, c.cfg.Region, c.cfg.Service)
	headers := signer.SignRequest("POST", urlStr, bodyBytes)

	log.Printf("[QUERY] Headers: %+v", headers)

	req, err := http.NewRequest("POST", urlStr, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body failed: %v", err)
	}

	log.Printf("[QUERY] Response Status: %d", resp.StatusCode)
	log.Printf("[QUERY] Response Body: %s", string(respBody))

	return json.Unmarshal(respBody, result)
}

type Signer struct {
	AK      string
	SK      string
	Region  string
	Service string
}

func NewSigner(ak, sk, region, service string) *Signer {
	return &Signer{
		AK:      ak,
		SK:      sk,
		Region:  region,
		Service: service,
	}
}

func (s *Signer) SignRequest(method, urlStr string, body []byte) map[string]string {
	parsedURL, _ := url.Parse(urlStr)

	now := time.Now()
	date := now.UTC().Format("20060102T150405Z")
	authDate := date[:8]

	payloadHash := hex.EncodeToString(hashSHA256(body))

	queryString := strings.Replace(parsedURL.RawQuery, "+", "%20", -1)

	signedHeaders := []string{"host", "x-date", "x-content-sha256", "content-type"}
	var headerList []string
	for _, header := range signedHeaders {
		if header == "host" {
			headerList = append(headerList, "host:"+parsedURL.Host)
		} else if header == "x-date" {
			headerList = append(headerList, "x-date:"+date)
		} else if header == "x-content-sha256" {
			headerList = append(headerList, "x-content-sha256:"+payloadHash)
		} else if header == "content-type" {
			headerList = append(headerList, "content-type:application/json")
		}
	}
	headerString := strings.Join(headerList, "\n")

	canonicalString := strings.Join([]string{
		method,
		"/",
		queryString,
		headerString + "\n",
		strings.Join(signedHeaders, ";"),
		payloadHash,
	}, "\n")

	hashedCanonicalString := hex.EncodeToString(hashSHA256([]byte(canonicalString)))

	credentialScope := authDate + "/" + s.Region + "/" + s.Service + "/request"

	signString := strings.Join([]string{
		"HMAC-SHA256",
		date,
		credentialScope,
		hashedCanonicalString,
	}, "\n")

	signedKey := getSignedKey(s.SK, authDate, s.Region, s.Service)
	signature := hex.EncodeToString(hmacSHA256(signedKey, signString))

	authorization := "HMAC-SHA256" +
		" Credential=" + s.AK + "/" + credentialScope +
		", SignedHeaders=" + strings.Join(signedHeaders, ";") +
		", Signature=" + signature

	return map[string]string{
		"X-Date":           date,
		"X-Content-Sha256": payloadHash,
		"Content-Type":     "application/json",
		"Authorization":    authorization,
	}
}

func hashSHA256(data []byte) []byte {
	h := sha256.New()
	h.Write(data)
	return h.Sum(nil)
}

func hmacSHA256(key []byte, content string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(content))
	return mac.Sum(nil)
}

func getSignedKey(secretKey, date, region, service string) []byte {
	kDate := hmacSHA256([]byte(secretKey), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "request")
	return kSigning
}

func GenerateTaskID() string {
	return uuid.New().String()
}

func IsVideoFunction(function string) bool {
	videoFuncs := map[string]bool{
		"t2v_720": true, "t2v_1080": true,
		"i2v_first_720": true, "i2v_first_1080": true,
		"i2v_first_tail_720": true, "i2v_first_tail_1080": true,
		"i2v_recamera_720": true,
		"ti2v_pro": true,
	}
	return videoFuncs[function]
}

func ParseVideoDuration(frames int) int {
	if frames <= 121 {
		return 5
	}
	return 10
}

func filterLocalURLs(urls []string) (localURLs []string, remoteURLs []string) {
	for _, url := range urls {
		if strings.HasPrefix(url, "http://localhost") ||
			strings.HasPrefix(url, "http://127.0.0.1") ||
			strings.HasPrefix(url, "/uploads/") {
			localURLs = append(localURLs, url)
		} else {
			remoteURLs = append(remoteURLs, url)
		}
	}
	return
}

func convertLocalImagesToBase64(urls []string) ([]string, error) {
	var result []string

	for _, urlStr := range urls {
		var filePath string
		if strings.HasPrefix(urlStr, "/uploads/") {
			filePath = strings.TrimPrefix(urlStr, "/uploads/")
		} else if strings.HasPrefix(urlStr, "http://localhost") || strings.HasPrefix(urlStr, "http://127.0.0.1") {
			parsed, err := url.Parse(urlStr)
			if err != nil {
				continue
			}
			filePath = strings.TrimPrefix(parsed.Path, "/uploads/")
		}

		if filePath == "" {
			continue
		}

		cfg := config.Get()
		fullPath := cfg.Upload.Path
		if !strings.HasSuffix(fullPath, string(filepath.Separator)) {
			fullPath += string(filepath.Separator)
		}
		fullPath += filePath

		data, err := os.ReadFile(fullPath)
		if err != nil {
			log.Printf("Failed to read file %s: %v", fullPath, err)
			continue
		}

		base64Str := base64.StdEncoding.EncodeToString(data)
		result = append(result, base64Str)
	}

	return result, nil
}

func functionSupportsBase64(function string) bool {
	base64SupportedFuncs := map[string]bool{
		"t2i_46":           true,
		"ti2v_pro":         true,
		"i2v_first_720":    true,
		"i2v_first_1080":   true,
		"i2v_first_tail_720":  true,
		"i2v_first_tail_1080": true,
		"i2v_recamera_720": true,
	}
	return base64SupportedFuncs[function]
}
