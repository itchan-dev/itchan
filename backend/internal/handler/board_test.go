package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/backend/internal/service"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockBoardService implements the service.BoardService interface
type MockBoardService struct {
	MockCreate func(creationData domain.BoardCreationData) error
	MockGet    func(shortName domain.BoardShortName, page int) (domain.Board, error)
	MockDelete func(shortName domain.BoardShortName) error
	MockGetAll func(user domain.User) ([]domain.Board, error)
}

func (m *MockBoardService) Create(creationData domain.BoardCreationData) error {
	if m.MockCreate != nil {
		return m.MockCreate(creationData)
	}
	return nil // Default behavior
}

func (m *MockBoardService) Get(shortName domain.BoardShortName, page int) (domain.Board, error) {
	if m.MockGet != nil {
		return m.MockGet(shortName, page)
	}
	return domain.Board{}, nil // Default behavior
}

func (m *MockBoardService) Delete(shortName domain.BoardShortName) error {
	if m.MockDelete != nil {
		return m.MockDelete(shortName)
	}
	return nil // Default behavior
}

func (m *MockBoardService) GetAll(user domain.User) ([]domain.Board, error) {
	if m.MockGetAll != nil {
		return m.MockGetAll(user)
	}
	return []domain.Board{}, nil // Default behavior
}

// Setup function to create handler with mock service
// Note: Config is not directly used by board handlers, so passing nil is acceptable for now.
func setupBoardTestHandler(boardService service.BoardService) (*Handler, *mux.Router) {
	h := &Handler{
		board: boardService,
		// auth, cfg, etc., could be added if needed by other parts of Handler
	}
	router := mux.NewRouter()
	// Define routes used in tests
	router.HandleFunc("/v1/boards", h.CreateBoard).Methods(http.MethodPost)
	router.HandleFunc("/v1/boards", h.GetBoards).Methods(http.MethodGet) // Added for GetBoards
	router.HandleFunc("/v1/{board}", h.GetBoard).Methods(http.MethodGet)
	router.HandleFunc("/v1/{board}", h.DeleteBoard).Methods(http.MethodDelete)

	return h, router
}

func TestCreateBoardHandler(t *testing.T) {
	route := "/v1/boards"
	validRequestBody := []byte(`{"name": "Test Board", "short_name": "tb"}`)
	requestBodyWithEmails := []byte(`{"name": "Test Board Email", "short_name": "tbe", "allowed_emails": ["test@example.com", "another@domain.org"]}`)
	expectedName := domain.BoardName("Test Board")
	expectedShortName := domain.BoardShortName("tb")
	expectedNameEmail := domain.BoardName("Test Board Email")
	expectedShortNameEmail := domain.BoardShortName("tbe")
	expectedEmails := &domain.Emails{"test@example.com", "another@domain.org"}

	t.Run("successful creation without emails", func(t *testing.T) {
		mockService := &MockBoardService{
			MockCreate: func(data domain.BoardCreationData) error {
				assert.Equal(t, expectedName, data.Name)
				assert.Equal(t, expectedShortName, data.ShortName)
				assert.Nil(t, data.AllowedEmails, "AllowedEmails should be nil when not provided")
				return nil
			},
		}
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodPost, route, validRequestBody)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Empty(t, rr.Body.String(), "Expected empty body on successful creation")
	})

	t.Run("successful creation with allowed emails", func(t *testing.T) {
		mockService := &MockBoardService{
			MockCreate: func(data domain.BoardCreationData) error {
				assert.Equal(t, expectedNameEmail, data.Name)
				assert.Equal(t, expectedShortNameEmail, data.ShortName)
				require.NotNil(t, data.AllowedEmails, "AllowedEmails should not be nil")
				assert.Equal(t, *expectedEmails, *data.AllowedEmails)
				return nil
			},
		}
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodPost, route, requestBodyWithEmails)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Empty(t, rr.Body.String())
	})

	t.Run("invalid JSON request body", func(t *testing.T) {
		mockService := &MockBoardService{} // Behavior doesn't matter
		_, router := setupBoardTestHandler(mockService)
		req := createRequest(t, http.MethodPost, route, []byte(`{"name": "Test Board",`)) // Incomplete JSON
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		// Assuming utils.WriteErrorAndStatusCode provides a meaningful message
		assert.Contains(t, rr.Body.String(), "Body is invalid json")
	})

	t.Run("missing required field (short_name)", func(t *testing.T) {
		mockService := &MockBoardService{}
		_, router := setupBoardTestHandler(mockService)
		invalidBody := []byte(`{"name": "Test Board Only"}`)
		req := createRequest(t, http.MethodPost, route, invalidBody)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		// Assuming validator returns a specific error message
		assert.Contains(t, rr.Body.String(), "Required fields missing") // Check if error mentions the missing field
	})

	t.Run("service error during creation", func(t *testing.T) {
		mockErr := errors.New("database connection failed")
		mockService := &MockBoardService{
			MockCreate: func(data domain.BoardCreationData) error {
				// Still check if data was passed correctly before returning error
				assert.Equal(t, expectedName, data.Name)
				assert.Equal(t, expectedShortName, data.ShortName)
				return mockErr
			},
		}
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodPost, route, validRequestBody)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		// Assuming utils.WriteErrorAndStatusCode writes the error message
		assert.Contains(t, rr.Body.String(), mockErr.Error())
	})
}

func TestGetBoardHandler(t *testing.T) {
	boardShortName := "testboard"
	routePrefix := "/v1/" + boardShortName
	expectedBoard := domain.Board{
		BoardMetadata: domain.BoardMetadata{Name: "Test Board", ShortName: boardShortName},
		Threads:       []domain.Thread{}, // Assuming empty threads for simplicity
	}

	t.Run("successful get with specific page", func(t *testing.T) {
		getPage := 2
		mockService := &MockBoardService{
			MockGet: func(shortName domain.BoardShortName, page int) (domain.Board, error) {
				assert.Equal(t, boardShortName, shortName)
				assert.Equal(t, getPage, page)
				return expectedBoard, nil
			},
		}
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodGet, routePrefix+"?page=2", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var actualBoard domain.Board
		err := json.Unmarshal(rr.Body.Bytes(), &actualBoard)
		require.NoError(t, err, "Failed to decode response body")
		assert.Equal(t, expectedBoard, actualBoard)
	})

	t.Run("successful get with default page (page 1)", func(t *testing.T) {
		defaultPage := 1
		mockService := &MockBoardService{
			MockGet: func(shortName domain.BoardShortName, page int) (domain.Board, error) {
				assert.Equal(t, boardShortName, shortName)
				assert.Equal(t, defaultPage, page, "Expected default page to be 1")
				return expectedBoard, nil
			},
		}
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodGet, routePrefix, nil) // No page query param
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var actualBoard domain.Board
		err := json.Unmarshal(rr.Body.Bytes(), &actualBoard)
		require.NoError(t, err, "Failed to decode response body")
		assert.Equal(t, expectedBoard, actualBoard)
	})

	t.Run("bad pagination parameter (non-integer)", func(t *testing.T) {
		mockService := &MockBoardService{} // Mock won't be called
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodGet, routePrefix+"?page=abc", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Page param should be integer")
	})

	t.Run("service error during get", func(t *testing.T) {
		mockErr := errors.New("board not found")
		mockService := &MockBoardService{
			MockGet: func(shortName domain.BoardShortName, page int) (domain.Board, error) {
				assert.Equal(t, boardShortName, shortName)
				return domain.Board{}, mockErr
			},
		}
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodGet, routePrefix+"?page=1", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// Assuming utils.WriteErrorAndStatusCode maps "not found" to 404, otherwise 500
		// Let's assume a generic error maps to 500 based on the broken test
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), mockErr.Error())
	})
}

func TestDeleteBoardHandler(t *testing.T) {
	boardShortName := "todelete"
	route := "/v1/" + boardShortName

	t.Run("successful deletion", func(t *testing.T) {
		mockService := &MockBoardService{
			MockDelete: func(shortName domain.BoardShortName) error {
				assert.Equal(t, boardShortName, shortName)
				return nil
			},
		}
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodDelete, route, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, rr.Body.String(), "Expected empty body on successful deletion")
	})

	t.Run("service error during deletion", func(t *testing.T) {
		mockErr := errors.New("permission denied")
		mockService := &MockBoardService{
			MockDelete: func(shortName domain.BoardShortName) error {
				assert.Equal(t, boardShortName, shortName)
				return mockErr
			},
		}
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodDelete, route, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// Assuming generic error maps to 500
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), mockErr.Error())
	})

	t.Run("board not found (simulated via service error)", func(t *testing.T) {
		// Often, a "not found" scenario in delete might still return OK or a specific error.
		// Let's simulate it returning an error that maps to a 404 or similar.
		// If the service always returns a generic error, this test might be identical to "service error".
		// We'll assume the service returns a specific error type or message that WriteErrorAndStatusCode handles.
		// For this example, let's stick to the generic error mapping based on previous tests.
		mockErr := errors.New("board does not exist")
		mockService := &MockBoardService{
			MockDelete: func(shortName domain.BoardShortName) error {
				assert.Equal(t, boardShortName, shortName)
				return mockErr // You might map this error to 404 in WriteErrorAndStatusCode
			},
		}
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodDelete, route, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code) // Adjust if 404 mapping exists
		assert.Contains(t, rr.Body.String(), mockErr.Error())
	})
}

func TestGetBoardsHandler(t *testing.T) {
	route := "/v1/boards"
	testUser := domain.User{Id: 1, Email: "test@example.com"}
	expectedBoards := []domain.Board{
		{BoardMetadata: domain.BoardMetadata{Name: "Board 1", ShortName: "b1"}},
		{BoardMetadata: domain.BoardMetadata{Name: "Board 2", ShortName: "b2"}},
	}

	userContextKey := mw.UserClaimsKey

	t.Run("successful retrieval", func(t *testing.T) {
		mockService := &MockBoardService{
			MockGetAll: func(user domain.User) ([]domain.Board, error) {
				assert.Equal(t, testUser, user)
				return expectedBoards, nil
			},
		}
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodGet, route, nil)
		// Inject user into context
		ctx := context.WithValue(req.Context(), userContextKey, &testUser)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var actualBoards []domain.Board
		err := json.Unmarshal(rr.Body.Bytes(), &actualBoards)
		require.NoError(t, err, "Failed to decode response body")
		assert.Equal(t, expectedBoards, actualBoards)
	})

	t.Run("unauthorized access (no user in context)", func(t *testing.T) {
		mockService := &MockBoardService{} // Mock won't be called
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodGet, route, nil) // No user injected
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Body.String(), "Not authorized")
	})

	t.Run("service error during retrieval", func(t *testing.T) {
		mockErr := errors.New("failed to query boards")
		mockService := &MockBoardService{
			MockGetAll: func(user domain.User) ([]domain.Board, error) {
				assert.Equal(t, testUser, user)
				return nil, mockErr
			},
		}
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodGet, route, nil)
		// Inject user into context
		ctx := context.WithValue(req.Context(), userContextKey, &testUser)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), mockErr.Error())
	})
}
