package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors"
	"github.com/itchan-dev/itchan/shared/utils"
)

// === Board Methods ===

func (c *APIClient) GetBoards(r *http.Request) ([]domain.Board, error) {
	var response api.BoardListResponse
	resp, err := c.do("GET", "/v1/boards", nil, r.Cookies()...)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend returned status %d", resp.StatusCode)
	}
	if err := utils.Decode(resp.Body, &response); err != nil {
		return nil, fmt.Errorf("cannot decode boards response: %w", err)
	}

	// Convert BoardMetadataResponse to Board
	boards := make([]domain.Board, len(response.Boards))
	for i, boardMeta := range response.Boards {
		boards[i] = domain.Board{
			BoardMetadata: boardMeta.BoardMetadata,
			Threads:       []*domain.Thread{}, // Empty threads for list view
		}
	}

	return boards, nil
}

func (c *APIClient) GetBoard(r *http.Request, shortName string, page int) (domain.Board, error) {
	var board domain.Board
	path := fmt.Sprintf("/v1/%s", shortName)
	if page > 1 {
		path = fmt.Sprintf("%s?page=%d", path, page)
	}

	resp, err := c.do("GET", path, nil, r.Cookies()...)
	if err != nil {
		return board, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return board, &internal_errors.ErrorWithStatusCode{
			Message: fmt.Sprintf("board /%s not found", shortName), StatusCode: http.StatusNotFound,
		}
	}
	if resp.StatusCode != http.StatusOK {
		return board, fmt.Errorf("backend returned status %d", resp.StatusCode)
	}
	if err := utils.Decode(resp.Body, &board); err != nil {
		return board, fmt.Errorf("cannot decode board response: %w", err)
	}
	return board, nil
}

func (c *APIClient) CreateBoard(r *http.Request, data api.CreateBoardRequest) error {
	jsonBody, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal board data: %w", err)
	}

	resp, err := c.do("POST", "/v1/admin/boards", bytes.NewBuffer(jsonBody), r.Cookies()...)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create board: %s", string(bodyBytes))
	}
	return nil
}

func (c *APIClient) DeleteBoard(r *http.Request, shortName string) error {
	path := fmt.Sprintf("/v1/admin/%s", shortName)
	resp, err := c.do("DELETE", path, nil, r.Cookies()...)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete board: %s", string(bodyBytes))
	}
	return nil
}
