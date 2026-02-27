package readfile

import (
	"os"
	"strings"
)

func ReadLinesNormalized(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	return strings.Split(normalized, "\n"), nil
}
