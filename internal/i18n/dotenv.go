package i18n

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
)

// DeveloperDefault resolves the developer locale without mutating the
// process environment. An existing process value wins over the dotenv file.
func DeveloperDefault(dotEnvPath string) (Locale, error) {
	if value, ok := os.LookupEnv(DefaultLocaleEnv); ok {
		return Parse(value)
	}

	file, err := os.Open(dotEnvPath)
	if errors.Is(err, fs.ErrNotExist) {
		return English, nil
	}
	if err != nil {
		return "", fmt.Errorf("i18n: open developer dotenv: %w", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		key, raw, ok := strings.Cut(scanner.Text(), "=")
		if !ok || strings.TrimSpace(key) != DefaultLocaleEnv {
			continue
		}
		value, err := parseDotEnvValue(raw)
		if err != nil {
			return "", fmt.Errorf("i18n: parse %s on dotenv line %d: %w", DefaultLocaleEnv, line, err)
		}
		return Parse(value)
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("i18n: read developer dotenv: %w", err)
	}
	return English, nil
}

func parseDotEnvValue(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	switch value[0] {
	case '\'':
		if len(value) < 2 || value[len(value)-1] != '\'' {
			return "", errors.New("unterminated single-quoted value")
		}
		return value[1 : len(value)-1], nil
	case '"':
		unquoted, err := strconv.Unquote(value)
		if err != nil {
			return "", errors.New("invalid double-quoted value")
		}
		return unquoted, nil
	default:
		return value, nil
	}
}
