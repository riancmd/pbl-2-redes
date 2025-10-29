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

func (u Users) UIDExists(uid string) bool {
	for _, v := range u.users {
		if v.UID == uid {
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

func (u *Users) SwitchCard(UID, CID string, newCard models.Card) error {
	i := 0

	for _, user := range u.users {
		if user.UID == UID {
			for _, card := range user.Deck {
				i++
				if card.CID == CID {
					user.Deck[i] = &newCard
					return nil
				}
			}
		}
	}

	return errors.New("card doesn't exist")

}

func (u *Users) GetDeck(UID string) []*models.Card {
	for _, user := range u.users {
		if user.UID == UID {
			return user.Deck
		}
	}
	return []*models.Card{}
}
