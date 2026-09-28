package service

import (
	"crypto/rand"
	"io"
	"math/big"
	"strings"
)

const (
	CodeLength = 10
	Alphabet   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"
)

func GenerateCode() (string, error) {
	return generateCode(rand.Reader)
}

func generateCode(reader io.Reader) (string, error) {
	var code [CodeLength]byte
	bound := big.NewInt(int64(len(Alphabet)))
	for i := range code {
		n, err := rand.Int(reader, bound)
		if err != nil {
			return "", err
		}
		code[i] = Alphabet[n.Int64()]
	}
	return string(code[:]), nil
}

func ValidCode(code string) bool {
	if len(code) != CodeLength {
		return false
	}
	for i := range code {
		if !strings.ContainsRune(Alphabet, rune(code[i])) {
			return false
		}
	}
	return true
}
