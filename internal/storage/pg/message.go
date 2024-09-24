package pg

import (
	"github.com/itchan-dev/itchan/internal/domain"

	_ "github.com/lib/pq"
)

// Saves message to db
func (s *Storage) CreateMessage(author *domain.User, text string, attachments []domain.Attachment, thread_id int64) (int64, error) {
	var id int64
	err := s.db.QueryRow("INSERT INTO messages(author_id, text, attachments, thread_id) VALUES($1, $2) RETURNING id", author.Id, text, attachments).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// // Get user from db
// //
// // If user doesn't exist, return error
// func (s *Storage) User(email string) (*domain.User, error) {
// 	result := s.db.QueryRow("SELECT id, email, pass_hash FROM users WHERE email = $1", email)

// 	var user domain.User
// 	err := result.Scan(&user.Id, &user.Email, &user.PassHash)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &user, nil
// }

// // Delete user from db
// //
// // If user doesn't exist, return error
// func (s *Storage) DeleteUser(email string) error {
// 	result, err := s.db.Exec("DELETE FROM users WHERE email = $1", email)
// 	if err != nil {
// 		return err
// 	}

// 	rowsDeleted, err := result.RowsAffected()
// 	if err != nil {
// 		return err
// 	}
// 	if rowsDeleted == 0 {
// 		return fmt.Errorf("no such user: %s", email)
// 	}

// 	return nil
// }
