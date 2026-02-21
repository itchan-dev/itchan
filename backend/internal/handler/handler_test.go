package handler

import (
	"bytes"
	"context"
	"image"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/itchan-dev/itchan/backend/internal/storage/fs"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createRequest(t *testing.T, method, url string, body []byte, cookies ...*http.Cookie) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, url, bytes.NewBuffer(body))
	for _, c := range cookies {
		req.AddCookie(c)
	}
	return req
}

func addUserToContext(req *http.Request, user *domain.User) *http.Request {
	ctx := context.WithValue(req.Context(), mw.UserClaimsKey, user)
	return req.WithContext(ctx)
}

func createMultipartFiles(t *testing.T, files []fileData) []*multipart.FileHeader {
	t.Helper()
	var result []*multipart.FileHeader

	for _, f := range files {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", f.name)
		require.NoError(t, err)
		_, err = part.Write(f.content)
		require.NoError(t, err)
		err = writer.Close()
		require.NoError(t, err)

		reader := multipart.NewReader(body, writer.Boundary())
		form, err := reader.ReadForm(1024 * 1024)
		require.NoError(t, err)

		for _, headers := range form.File {
			for _, header := range headers {
				if f.contentType != "" {
					header.Header.Set("Content-Type", f.contentType)
				}
				result = append(result, header)
			}
		}
	}
	return result
}

type fileData struct {
	name        string
	content     []byte
	contentType string
}

type MockMediaStorage struct {
	readFunc func(filePath string) (io.ReadCloser, error)
}

func (m *MockMediaStorage) SaveFile(fileData io.Reader, boardID, threadID, originalFilename string) (string, error) {
	return "", nil
}

func (m *MockMediaStorage) SaveImage(img image.Image, format, boardID, threadID, originalFilename string) (string, int64, error) {
	return "", 0, nil
}

func (m *MockMediaStorage) MoveFile(sourcePath, boardID, threadID, filename string) (string, error) {
	return "", nil
}

func (m *MockMediaStorage) SaveThumbnail(data io.Reader, originalRelativePath string) (string, error) {
	return "", nil
}

func (m *MockMediaStorage) Read(filePath string) (io.ReadCloser, error) {
	if m.readFunc != nil {
		return m.readFunc(filePath)
	}
	return nil, io.EOF
}

func (m *MockMediaStorage) DeleteFile(filePath string) error {
	return nil
}

func (m *MockMediaStorage) DeleteThread(boardID, threadID string) error {
	return nil
}

func (m *MockMediaStorage) DeleteBoard(boardID string) error {
	return nil
}

var _ fs.MediaStorage = (*MockMediaStorage)(nil)

func TestWriteJSON(t *testing.T) {
	t.Run("valid JSON", func(t *testing.T) {
		rr := httptest.NewRecorder()
		writeJSON(rr, map[string]string{"message": "hello"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
		assert.Equal(t, `{"message":"hello"}`+"\n", rr.Body.String())
	})

	t.Run("encoding error", func(t *testing.T) {
		rr := httptest.NewRecorder()
		log.SetOutput(io.Discard)
		defer log.SetOutput(os.Stderr)

		writeJSON(rr, make(chan int))

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Equal(t, "Internal error\n", rr.Body.String())
	})
}
