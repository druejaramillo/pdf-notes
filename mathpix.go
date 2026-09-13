package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const mathpixAPIURL = "https://api.mathpix.com/v3"

type mathpixClient struct {
	appID        string
	appKey       string
	baseURL      string
	httpClient   *http.Client
	pollInterval time.Duration
	report       func(string)
}

func defaultMathpixClient(appID, appKey string) mathpixClient {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.ResponseHeaderTimeout = 30 * time.Second

	return mathpixClient{
		appID:        appID,
		appKey:       appKey,
		baseURL:      mathpixAPIURL,
		httpClient:   &http.Client{Transport: transport, Timeout: 2 * time.Minute},
		pollInterval: 500 * time.Millisecond,
	}
}

func (c mathpixClient) convert(ctx context.Context, path string) (markdown string, err error) {
	pdfID, err := c.upload(ctx, path)
	if err != nil {
		return "", err
	}
	c.progress("Mathpix accepted the PDF; waiting for conversion...")
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		c.progress("Removing the uploaded PDF from Mathpix...")
		cleanupErr := c.delete(cleanupCtx, pdfID)
		if cleanupErr != nil && err == nil {
			err = fmt.Errorf("delete Mathpix PDF %q: %w", pdfID, cleanupErr)
		}
	}()

	if err := c.waitForCompletion(ctx, pdfID); err != nil {
		return "", err
	}
	return c.downloadMarkdown(ctx, pdfID)
}

func (c mathpixClient) upload(ctx context.Context, path string) (string, error) {
	c.progress("Preparing PDF upload...")
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open PDF %q: %w", path, err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("options_json", `{"idiomatic_eqn_arrays":true}`); err != nil {
		return "", fmt.Errorf("build Mathpix request: %w", err)
	}
	part, err := writer.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return "", fmt.Errorf("build Mathpix request: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", fmt.Errorf("read PDF %q: %w", path, err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("finalize Mathpix request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/pdf", &body)
	if err != nil {
		return "", fmt.Errorf("create Mathpix upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.setAuth(req)
	c.progress("Sending PDF to Mathpix...")
	response, err := c.do(req)
	if err != nil {
		return "", fmt.Errorf("upload PDF to Mathpix: %w", err)
	}
	defer response.Body.Close()

	var result struct {
		PDFID string `json:"pdf_id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode Mathpix upload response: %w", err)
	}
	if result.PDFID == "" {
		return "", fmt.Errorf("decode Mathpix upload response: missing pdf_id")
	}
	return result.PDFID, nil
}

func (c mathpixClient) waitForCompletion(ctx context.Context, pdfID string) error {
	pollInterval := c.pollInterval
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for Mathpix PDF %q: %w", pdfID, ctx.Err())
		case <-ticker.C:
			status, err := c.status(ctx, pdfID)
			if err != nil {
				return err
			}
			switch status {
			case "completed":
				c.progress("Mathpix conversion completed; downloading Markdown...")
				return nil
			case "error":
				return fmt.Errorf("Mathpix failed to process PDF %q", pdfID)
			}
		}
	}
}

func (c mathpixClient) status(ctx context.Context, pdfID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/pdf/"+pdfID, nil)
	if err != nil {
		return "", fmt.Errorf("create Mathpix status request: %w", err)
	}
	c.setAuth(req)
	response, err := c.do(req)
	if err != nil {
		return "", fmt.Errorf("check Mathpix PDF %q: %w", pdfID, err)
	}
	defer response.Body.Close()

	var result struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode Mathpix status response: %w", err)
	}
	return result.Status, nil
}

func (c mathpixClient) downloadMarkdown(ctx context.Context, pdfID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/pdf/"+pdfID+".mmd", nil)
	if err != nil {
		return "", fmt.Errorf("create Mathpix Markdown request: %w", err)
	}
	c.setAuth(req)
	response, err := c.do(req)
	if err != nil {
		return "", fmt.Errorf("download Mathpix Markdown for PDF %q: %w", pdfID, err)
	}
	defer response.Body.Close()
	markdown, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("read Mathpix Markdown response: %w", err)
	}
	return string(markdown), nil
}

func (c mathpixClient) delete(ctx context.Context, pdfID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/pdf/"+pdfID, nil)
	if err != nil {
		return err
	}
	c.setAuth(req)
	response, err := c.do(req)
	if err != nil {
		return err
	}
	return response.Body.Close()
}

func (c mathpixClient) setAuth(req *http.Request) {
	req.Header.Set("app_id", c.appID)
	req.Header.Set("app_key", c.appKey)
}

func (c mathpixClient) do(req *http.Request) (*http.Response, error) {
	response, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return response, nil
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 4096))
	response.Body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("Mathpix returned HTTP %d", response.StatusCode)
	}
	message := strings.TrimSpace(string(body))
	if message == "" {
		return nil, fmt.Errorf("Mathpix returned HTTP %d", response.StatusCode)
	}
	return nil, fmt.Errorf("Mathpix returned HTTP %d: %s", response.StatusCode, message)
}

func (c mathpixClient) progress(message string) {
	if c.report != nil {
		c.report(message)
	}
}
