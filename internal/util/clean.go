package util

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"unicode/utf8"
)

const maxBinaryCheckBytes = 512

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

var charReplacementMap = map[string]string{
	"\u00a9": "©", "\u2018": "'", "\u2019": "'", "\u201C": "\"",
	"\u201D": "\"", "\u2013": "-", "\u2014": "--", "\u2026": "...",
	"\u00a0": " ", "\u0096": "-", "\u0097": "--", "\u0091": "'",
	"\u0092": "'", "\u0093": "\"", "\u0094": "\"", "\u2122": "™",
	"\u00AE": "®", "\u2022": "•",
}

func IsLikelyBinary(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	buffer := make([]byte, maxBinaryCheckBytes)
	n, err := file.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}

	return bytes.Contains(buffer[:n], []byte{0}), nil
}

func CleanFileContent(fileContentBytes []byte, src string) (string, error) {
	fileContentBytes = bytes.TrimPrefix(fileContentBytes, utf8BOM)

	if !utf8.Valid(fileContentBytes) {
		log.Printf("WARN: %s invalid UTF-8, replacing invalid chars", src)
		fileContentBytes = bytes.ToValidUTF8(fileContentBytes, []byte(string(utf8.RuneError)))
	}

	str := string(fileContentBytes)
	for bad, good := range charReplacementMap {
		str = strings.ReplaceAll(str, bad, good)
	}

	if !utf8.ValidString(str) {
		log.Printf("ERR: %s still invalid after cleaning", src)
		return "", fmt.Errorf("invalid UTF-8 after replacements: %s", src)
	}
	return str, nil
}
