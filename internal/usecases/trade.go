package usecases

import "pbl-2-redes/internal/models"

// Troca carta de acordo com ID
func (u UseCases) Trade(UID, CID string, card models.Card) error {
	err := u.sync.TradeCard(UID, CID, card)

	err = u.repos.User.SwitchCard(UID, CID, card)
	if err != nil {
		return err
	}

	return nil
}
