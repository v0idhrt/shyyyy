package models

// ============================================================
// User Model
// ============================================================

type User struct {
	ID        string `json:"id"`
	Login     string `json:"login"`
	Password  string `json:"password"`
	FIO       string `json:"fio"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	BirthDate string `json:"birth_date"`
	Address   string `json:"address"`
	CreatedAt string `json:"created_at"`
}
