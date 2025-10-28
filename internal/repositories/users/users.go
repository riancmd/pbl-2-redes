package users

import (
	"errors"
	"pbl-2-redes/internal/models"
)

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

func (u *Users) CheckPassword(usern string, password string) (bool, error) {
	for _, user := range u.users {
		if user.Username == usern && user.Password == password {
			return true, nil
		}
	}
	return false, errors.New("password incorrect")
}
