package pg

import (
	_ "github.com/lib/pq"
)

// Saves message to db
func (s *Storage) CreateBoard(name, shortName string) error {
	_, err := s.db.Exec("INSERT INTO boards(name, short_name) VALUES($1, $2)", name, shortName)
	return err
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
