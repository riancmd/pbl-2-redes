package cluster

import (
	"encoding/json"
	"errors"
	"net/http"
	"pbl-2-redes/internal/models"
	"strconv"
)

// Ordena aos peers que façam algo
func (c *Client) BroadcastToPeers(action string, info string) error {
	return nil
}

// Ordena que devolvam uma mão pro servidor líder
func (c *Client) GetHand(UID string) ([]*models.Card, error) {
	// Verificar qual servidor é
	var peer = 0
	for _, p := range c.peers {
		exists, _ := c.uidExists(p, UID)

		if exists {
			peer = p
		}

	}

	if peer == 0 {
		return []*models.Card{}, errors.New("user doesn't exist")
	}

	// dá um GET nas cartas
	resp, err := c.httpClient.Get("http://localhost:" + strconv.Itoa(peer) + "/internal/users/{uid}/hand") // Endereço temporário, resolver

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var hand []*models.Card

	if resp.StatusCode == http.StatusOK {
		json.NewDecoder(resp.Body).Decode(&hand)
		return hand, nil
	}

	if resp.StatusCode == http.StatusForbidden {
		json.NewDecoder(resp.Body).Decode(&hand)
		return hand, errors.New("user doesn't have enough cards")
	}

	hand = []*models.Card{}

	return hand, errors.New("user doesn't exist")
}

func (c *Client) uidExists(peer int, uid string) (bool, error) {
	resp, err := c.httpClient.Get("http://localhost:" + strconv.Itoa(peer) + "/internal/users/{" + uid + "}") // Endereço temporário, resolver

	if err != nil {
		return false, err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusFound {
		return true, nil
	}

	return false, nil

}
