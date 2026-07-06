package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// loadDotenv reads simple KEY=VALUE pairs from path and applies them to the
// process environment, without overriding variables already set (so real
// environment variables always take precedence over the .env file). A
// missing file is not an error, since the .env file is optional.
func loadDotenv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("%s:%d: erwarte KEY=VALUE", path, lineNo)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)

		if key == "" {
			return fmt.Errorf("%s:%d: leerer Schlüssel", path, lineNo)
		}

		if _, alreadySet := os.LookupEnv(key); alreadySet {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return scanner.Err()
}
