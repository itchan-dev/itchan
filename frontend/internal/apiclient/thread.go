package apiclient

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors"
	"github.com/itchan-dev/itchan/shared/utils"
)

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

// === Thread & Message Methods ===

func (c *APIClient) GetThread(r *http.Request, shortName, threadID string) (domain.Thread, error) {
	var thread domain.Thread
	path := fmt.Sprintf("/v1/%s/%s", shortName, threadID)
	resp, err := c.do("GET", path, nil, r.Cookies()...)
	if err != nil {
		return thread, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return thread, &internal_errors.ErrorWithStatusCode{
			Message: fmt.Sprintf("thread /%s/%s not found or access denied", shortName, threadID), StatusCode: resp.StatusCode,
		}
	}

	if err := utils.Decode(resp.Body, &thread); err != nil {
		return thread, fmt.Errorf("cannot decode thread response: %w", err)
	}
	return thread, nil
}

// postMultipartRequest sends a multipart/form-data POST request with JSON payload and optional file attachments
func (c *APIClient) postMultipartRequest(path string, data any, multipartForm *multipart.Form, cookies []*http.Cookie) ([]byte, int, error) {
	// Create multipart writer
	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)

	// Start a goroutine to write the multipart data
	go func() {
		defer pipeWriter.Close()
		defer writer.Close()

		// Write JSON payload
		jsonData, err := json.Marshal(data)
		if err != nil {
			pipeWriter.CloseWithError(err)
			return
		}

		if err := writer.WriteField("json", string(jsonData)); err != nil {
			pipeWriter.CloseWithError(err)
			return
		}

		// Write files if present
		if multipartForm != nil && len(multipartForm.File["attachments"]) > 0 {
			for _, fileHeaders := range multipartForm.File["attachments"] {
				file, err := fileHeaders.Open()
				if err != nil {
					pipeWriter.CloseWithError(err)
					return
				}
				defer file.Close()

				// Create part with proper Content-Type header
				h := make(textproto.MIMEHeader)
				h.Set("Content-Disposition",
					fmt.Sprintf(`form-data; name="attachments"; filename="%s"`,
						escapeQuotes(fileHeaders.Filename)))

				// Preserve the original Content-Type from the uploaded file
				contentType := fileHeaders.Header.Get("Content-Type")
				if contentType != "" {
					h.Set("Content-Type", contentType)
				}

				part, err := writer.CreatePart(h)
				if err != nil {
					pipeWriter.CloseWithError(err)
					return
				}

				if _, err := io.Copy(part, file); err != nil {
					pipeWriter.CloseWithError(err)
					return
				}
			}
		}
	}()

	// Create request
	req, err := http.NewRequest("POST", c.BaseURL+path, pipeReader)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create API request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Add cookies
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("backend unavailable: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	return bodyBytes, resp.StatusCode, nil
}

func (c *APIClient) CreateThread(r *http.Request, shortName string, data api.CreateThreadRequest, multipartForm *multipart.Form) (string, error) {
	path := fmt.Sprintf("/v1/%s", shortName)
	bodyBytes, statusCode, err := c.postMultipartRequest(path, data, multipartForm, r.Cookies())
	if err != nil {
		return "", err
	}
	if statusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to create thread: %s", string(bodyBytes))
	}
	return string(bodyBytes), nil
}

func (c *APIClient) CreateReply(r *http.Request, shortName, threadID string, data api.CreateMessageRequest, multipartForm *multipart.Form) error {
	path := fmt.Sprintf("/v1/%s/%s", shortName, threadID)
	bodyBytes, statusCode, err := c.postMultipartRequest(path, data, multipartForm, r.Cookies())
	if err != nil {
		return err
	}
	if statusCode != http.StatusCreated {
		return fmt.Errorf("failed to create reply: %s", string(bodyBytes))
	}
	return nil
}


func (c *APIClient) DeleteThread(r *http.Request, shortName, threadID string) error {
	path := fmt.Sprintf("/v1/admin/%s/%s", shortName, threadID)
	resp, err := c.do("DELETE", path, nil, r.Cookies()...)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete thread: %s", string(bodyBytes))
	}
	return nil
}
