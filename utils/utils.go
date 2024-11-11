package utils

func IsProfaneWord(word string) bool {
	profaneWords := []string{"kerfuffle", "sharbert", "fornax"}
	for _, profane := range profaneWords {
		if word == profane {
			return true
		}
	}

	return false
}
