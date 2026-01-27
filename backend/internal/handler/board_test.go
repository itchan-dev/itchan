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
	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	return nil
}

func (m *MockBoardService) Get(shortName domain.BoardShortName, page int) (domain.Board, error) {
	if m.MockGet != nil {
		return m.MockGet(shortName, page)
	}
	return domain.Board{}, nil
}

func (m *MockBoardService) Delete(shortName domain.BoardShortName) error {
	if m.MockDelete != nil {
		return m.MockDelete(shortName)
	}
	return nil
}

func (m *MockBoardService) GetAll(user domain.User) ([]domain.Board, error) {
	if m.MockGetAll != nil {
		return m.MockGetAll(user)
	}
	return []domain.Board{}, nil
}

func setupBoardTestHandler(boardService service.BoardService) (*Handler, *mux.Router) {
	h := &Handler{
		board: boardService,
	}
	router := mux.NewRouter()
	router.HandleFunc("/v1/boards", h.CreateBoard).Methods(http.MethodPost)
	router.HandleFunc("/v1/boards", h.GetBoards).Methods(http.MethodGet)
	router.HandleFunc("/v1/{board}", h.GetBoard).Methods(http.MethodGet)
	router.HandleFunc("/v1/{board}", h.DeleteBoard).Methods(http.MethodDelete)

	return h, router
}

func TestCreateBoardHandler(t *testing.T) {
	route := "/v1/boards"
	validRequestBody := []byte(`{"name": "Test Board", "short_name": "tb"}`)
	requestBodyWithEmails := []byte(`{"name": "Test Board Email", "short_name": "tbe", "allowed_emails": ["test@example.com", "another@domain.org"]}`)

	t.Run("successful creation without emails", func(t *testing.T) {
		mockService := &MockBoardService{
			MockCreate: func(data domain.BoardCreationData) error {
				assert.Equal(t, domain.BoardName("Test Board"), data.Name)
				assert.Equal(t, domain.BoardShortName("tb"), data.ShortName)
				assert.Nil(t, data.AllowedEmails)
				return nil
			},
		}
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodPost, route, validRequestBody)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Empty(t, rr.Body.String())
	})

	t.Run("successful creation with allowed emails", func(t *testing.T) {
		expectedEmails := &domain.Emails{"test@example.com", "another@domain.org"}
		mockService := &MockBoardService{
			MockCreate: func(data domain.BoardCreationData) error {
				require.NotNil(t, data.AllowedEmails)
				assert.Equal(t, *expectedEmails, *data.AllowedEmails)
				return nil
			},
		}
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodPost, route, requestBodyWithEmails)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("validation error", func(t *testing.T) {
		_, router := setupBoardTestHandler(&MockBoardService{})
		req := createRequest(t, http.MethodPost, route, []byte(`{"name": "Test Board",`))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("database connection failed")
		mockService := &MockBoardService{
			MockCreate: func(data domain.BoardCreationData) error {
				return mockErr
			},
		}
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodPost, route, validRequestBody)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestGetBoardHandler(t *testing.T) {
	boardShortName := "testboard"
	routePrefix := "/v1/" + boardShortName
	expectedBoard := domain.Board{
		BoardMetadata: domain.BoardMetadata{Name: "Test Board", ShortName: boardShortName},
		Threads:       []*domain.Thread{},
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
		require.NoError(t, err)
		assert.Equal(t, expectedBoard, actualBoard)
	})

	t.Run("successful get with default page", func(t *testing.T) {
		mockService := &MockBoardService{
			MockGet: func(shortName domain.BoardShortName, page int) (domain.Board, error) {
				assert.Equal(t, 1, page)
				return expectedBoard, nil
			},
		}
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodGet, routePrefix, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("invalid page parameter", func(t *testing.T) {
		_, router := setupBoardTestHandler(&MockBoardService{})

		req := createRequest(t, http.MethodGet, routePrefix+"?page=abc", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("board not found")
		mockService := &MockBoardService{
			MockGet: func(shortName domain.BoardShortName, page int) (domain.Board, error) {
				return domain.Board{}, mockErr
			},
		}
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodGet, routePrefix+"?page=1", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
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
		assert.Empty(t, rr.Body.String())
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("permission denied")
		mockService := &MockBoardService{
			MockDelete: func(shortName domain.BoardShortName) error {
				return mockErr
			},
		}
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodDelete, route, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestGetBoardsHandler(t *testing.T) {
	route := "/v1/boards"
	testUser := domain.User{Id: 1}
	expectedBoards := []domain.Board{
		{BoardMetadata: domain.BoardMetadata{Name: "Board 1", ShortName: "b1"}},
		{BoardMetadata: domain.BoardMetadata{Name: "Board 2", ShortName: "b2"}},
	}

	t.Run("successful retrieval", func(t *testing.T) {
		mockService := &MockBoardService{
			MockGetAll: func(user domain.User) ([]domain.Board, error) {
				assert.Equal(t, testUser, user)
				return expectedBoards, nil
			},
		}
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodGet, route, nil)
		ctx := context.WithValue(req.Context(), mw.UserClaimsKey, &testUser)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response api.BoardListResponse
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Len(t, response.Boards, len(expectedBoards))
		for i, boardMeta := range response.Boards {
			assert.Equal(t, expectedBoards[i].BoardMetadata, boardMeta.BoardMetadata)
		}
	})

	t.Run("unauthorized access", func(t *testing.T) {
		_, router := setupBoardTestHandler(&MockBoardService{})

		req := createRequest(t, http.MethodGet, route, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("failed to query boards")
		mockService := &MockBoardService{
			MockGetAll: func(user domain.User) ([]domain.Board, error) {
				return nil, mockErr
			},
		}
		_, router := setupBoardTestHandler(mockService)

		req := createRequest(t, http.MethodGet, route, nil)
		ctx := context.WithValue(req.Context(), mw.UserClaimsKey, &testUser)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}
