package usecases

import (
	"errors"
	"log/slog"
	"math/rand"
	"pbl-2-redes/internal/models"
	"time"
)

// Retorna lista com todos os usuários
func (u *UseCases) GetAllUsers() []models.User {
	users := u.repos.User.GetAll()
	return users
}

// Acrescenta novo usuário na lista de usuários do servidor
func (u *UseCases) AddUser(newUser models.CreateUserRequest) error {
	exists := u.repos.User.UserExists(newUser.Username)

	if exists {
		slog.Error("this user already exists", "username", newUser.Username)
		return errors.New("user already exists")
	}

	repoReq := models.User{
		UID:         newUser.UID,
		Username:    newUser.Username,
		Password:    newUser.Password,
		Deck:        make([]*models.Card, 0),
		CreatedAt:   time.Now(),
		LastLogin:   time.Now(),
		TotalWins:   0,
		TotalLosses: 0,
		IsInBattle:  false,
	}

	// Verifica se existe em outro servidor
	err := u.sync.UserNew(newUser.Username)

	if err != nil {
		slog.Error("this user already exists", "username", newUser.Username)
		return err
	}

	u.repos.User.Add(repoReq)

	return nil
}

// Verifica se usuário existe
func (u *UseCases) UserExists(username string) bool {
	exists := u.repos.User.UserExists(username)
	return exists
}

// Verifica se usuário existe
func (u *UseCases) UIDExists(uid string) bool {
	exists := u.repos.User.UIDExists(uid)
	return exists
}

// Faz login em usuário
func (u *UseCases) Login(user string, password string) (bool, error) {
	login, err := u.repos.User.CheckPassword(user, password)

	if !login {
		return false, err
	}

	return true, nil
}

// Pega a mão do usuário
func (u *UseCases) GetHand(uid string) ([]*models.Card, error) {
	deck := u.repos.User.GetDeck(uid)
	// verifica se user tem carta
	if len(deck) < 10 {
		return []*models.Card{}, errors.New("user doesn't have enough cards")
	}

	hand := make([]*models.Card, len(deck))
	copy(hand, deck)

	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	random.Shuffle(len(hand), func(i, j int) { hand[i], hand[j] = hand[j], hand[i] })
	if len(hand) > 10 {
		hand = hand[:10]
	}
	return hand, nil
}

//
