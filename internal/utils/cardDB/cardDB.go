package cardDB

import (
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"os"
	"pbl-2-redes/internal/models"
	"time"
)

const CARDS_PER_BOOSTER = 5

type CardDB struct {
	database []models.Card
}

func New() *CardDB {
	return &CardDB{database: make([]models.Card, 0)}
}

// passa as cartas que estão no arquivo JSON para um map no próprio programa
func (cd CardDB) InitializeCardsFromJSON(filename string) (map[string]models.Card, error) {
	file, error := os.ReadFile(filename)
	if error != nil {
		return nil, errors.New("reading file error")
	}

	// base de dados de cartas, definido no types
	// cardDB contém um map de CID e cartas
	var cardDB models.CardDB
	error = json.Unmarshal(file, &cardDB)
	if error != nil {
		return nil, errors.New("unmarshal error")
	}

	return cardDB.Cards, nil
}

// carrega quais as cartas existentes no JSON
func (cd CardDB) LoadCardsFromFile(filename string) (map[string]models.Card, error) {
	cards, error := cd.InitializeCardsFromJSON(filename)
	if error != nil {
		return map[string]models.Card{}, error
	}

	return cards, nil
}

// calcula quantidade de cópias de cada carta
// coloco a quantidade de boosters que quero
// retorno: map com quantas de cada carta
func (cd CardDB) CalculateCardCopies(glossary map[string]models.Card, boostersCount int) map[string]int {
	// conta cartas por tipo
	remCards := []string{}
	nremCards := []string{}
	pillCards := []string{}

	// acrescento cada CID nos slices de string contendo os CIDs
	for cid, card := range glossary {
		switch card.CardType {
		case models.REM:
			remCards = append(remCards, cid)
		case models.NREM:
			nremCards = append(nremCards, cid)
		case models.Pill:
			pillCards = append(pillCards, cid)
		}
	}

	totalCardsNeeded := boostersCount * CARDS_PER_BOOSTER

	// faço a distribuição por raridade, considerando
	// 50% das cartas são comuns
	// 40% das cartas são incomuns
	// 10% das cartas são raras
	commonCards := int(float64(totalCardsNeeded) * 0.5)
	uncommonCards := int(float64(totalCardsNeeded) * 0.4)
	rareCards := int(float64(totalCardsNeeded) * 0.1)

	copies := make(map[string]int) // map que contém quantidade de cada carta

	// agora, calculo quantas cópias serão necessárias para cada carta
	for cid, card := range glossary { // passo por cada carta no glossário
		var neededCopies float64

		switch card.CardRarity {
		case models.Comum:
			// divido as raridades proporcionalmente aos cardType
			commonByType := float64(commonCards) / 3.0 // rem, nrem, pill
			switch card.CardType {
			case models.REM:
				neededCopies = commonByType / float64(len(remCards))
			case models.NREM:
				neededCopies = commonByType / float64(len(nremCards))
			case models.Pill:
				neededCopies = commonByType / float64(len(pillCards))
			}
		case models.Incomum:
			uncommonByType := float64(uncommonCards) / 3.0
			switch card.CardType {
			case models.REM:
				neededCopies = uncommonByType / float64(len(remCards))
			case models.NREM:
				neededCopies = uncommonByType / float64(len(nremCards))
			case models.Pill:
				neededCopies = uncommonByType / float64(len(pillCards))
			}
		case models.Rara:
			rareByType := float64(rareCards) / 3.0
			switch card.CardType {
			case models.REM:
				neededCopies = rareByType / float64(len(remCards))
			case models.NREM:
				neededCopies = rareByType / float64(len(nremCards))
			case models.Pill:
				neededCopies = rareByType / float64(len(pillCards))
			}
		}

		finalCopies := int(math.Round(neededCopies))

		// garante pelo menos 1 cópia de cada carta
		if neededCopies < 1 {
			neededCopies = 1
		}

		copies[cid] = finalCopies
	}

	// agora, verifica se o calculado realmente bate com a quantidade
	totalCalculated := 0
	for _, quantity := range copies {
		totalCalculated += quantity
	}

	difference := totalCalculated - totalCardsNeeded

	if difference > 0 {
		// precisa remover cartas, então remove das que têm mais cópias
		for i := 0; i < difference; i++ {
			maxCopies := 1
			maxCardID := ""

			for cardID, quantity := range copies {
				if quantity > maxCopies {
					maxCopies = quantity
					maxCardID = cardID
				}
			}

			if maxCardID != "" {
				copies[maxCardID]--
			}
		}
	} else if difference < 0 {
		// casoo precise adicionar, adiciona nas que têm menos cópias
		for i := 0; i < -difference; i++ {
			minCopies := math.MaxInt32
			minCardID := ""

			for cardID, quantity := range copies {
				if quantity < minCopies {
					minCopies = quantity
					minCardID = cardID
				}
			}

			if minCardID != "" {
				copies[minCardID]++
			}
		}
	}

	return copies
}

// crio um "pool" de cartas baseado nas cópias calculadas
// esse bolo de cartas é utilizado na hora de criar os boosters
func (cd CardDB) CreateCardPool(glossary map[string]models.Card, copies map[string]int) []models.Card {
	var pool []models.Card

	for cid, quantity := range copies {
		card := glossary[cid]
		for i := 0; i < quantity; i++ {
			pool = append(pool, card)
		}
	}

	return pool
}

// possui a lógica para organizar os boosters individualmente
// a partir de um cardPool de forma aleatória
// cardPool: tem todas as unidades de cartas geradas já com raridade sortida
// boostersCount: quantidade de boosters a serem criados
func (cd CardDB) CreateBoosters(cardPool []models.Card, boostersCount int) ([]models.Booster, error) {
	// a estrutura vault guarda todos os boosters
	vault := make([]models.Booster, 0)
	Generator := rand.New(rand.NewSource(time.Now().UnixNano())) // gerador

	// embaralho o pool com o generator
	Generator.Shuffle(len(cardPool), func(i, j int) {
		cardPool[i], cardPool[j] = cardPool[j], cardPool[i]
	})
	println("\n Iniciando criação dos boosters...")
	// crio os boosters individualmente
	for i := 0; i < boostersCount; i++ {
		booster := models.Booster{
			BID:     i,
			Booster: make([]models.Card, 0, CARDS_PER_BOOSTER),
		}

		// pego as próximas n cartas do pool
		startIndex := i * CARDS_PER_BOOSTER
		endIndex := startIndex + CARDS_PER_BOOSTER

		// verifico se ainda tá dentro do tamanho do pool
		if endIndex > len(cardPool) {
			endIndex = len(cardPool)
		}

		// acrescento as cartas
		for j := startIndex; j < endIndex; j++ {
			booster.Booster = append(booster.Booster, cardPool[j])
		}

		vault = append(vault, booster)
	}
	return vault, nil
}
