package users

import "pbl-2-redes/internal/models"

type Users struct {
	users []models.User
}

func New() *Users {
	return &Users{users: make([]models.User, 0)}
}

func (u Users) GetAll() []models.User {
	return u.users
}

func (u Users) UserExists(username string) bool {
	for _, v := range u.users {
		if v.Username == username {
			return true
		}
	}
	return false
}

func (u *Users) Add(newUser models.User) {
	u.users = append(u.users, newUser)
}
