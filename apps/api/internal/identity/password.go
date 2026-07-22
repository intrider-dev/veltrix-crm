package identity

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

type PasswordHasher struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
	gate        chan struct{}
}

func NewPasswordHasher(maxConcurrent int) *PasswordHasher {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &PasswordHasher{
		memory:      32 * 1024,
		iterations:  2,
		parallelism: 1,
		saltLength:  16,
		keyLength:   32,
		gate:        make(chan struct{}, maxConcurrent),
	}
}

func (hasher *PasswordHasher) Hash(password string) (string, error) {
	if len(password) < 8 || len(password) > 1024 {
		return "", errors.New("password length must be between 8 and 1024 bytes")
	}
	salt := make([]byte, hasher.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := hasher.derive(password, salt, hasher.memory, hasher.iterations, hasher.parallelism, hasher.keyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		hasher.memory,
		hasher.iterations,
		hasher.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func (hasher *PasswordHasher) Verify(password, encoded string) (bool, error) {
	if len(password) < 1 || len(password) > 1024 {
		return false, nil
	}
	parameters, salt, expected, err := parsePasswordHash(encoded)
	if err != nil {
		return false, err
	}
	actual := hasher.derive(password, salt, parameters.memory, parameters.iterations, parameters.parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func (hasher *PasswordHasher) derive(password string, salt []byte, memory, iterations uint32, parallelism uint8, keyLength uint32) []byte {
	hasher.gate <- struct{}{}
	defer func() { <-hasher.gate }()
	return argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)
}

type hashParameters struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

func parsePasswordHash(encoded string) (hashParameters, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return hashParameters{}, nil, nil, errors.New("invalid password hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return hashParameters{}, nil, nil, errors.New("unsupported argon2 version")
	}
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return hashParameters{}, nil, nil, errors.New("invalid argon2 parameters")
	}
	values := make(map[string]uint64, 3)
	for _, pair := range params {
		keyValue := strings.SplitN(pair, "=", 2)
		if len(keyValue) != 2 {
			return hashParameters{}, nil, nil, errors.New("invalid argon2 parameter")
		}
		value, err := strconv.ParseUint(keyValue[1], 10, 32)
		if err != nil {
			return hashParameters{}, nil, nil, errors.New("invalid argon2 parameter")
		}
		values[keyValue[0]] = value
	}
	parameters := hashParameters{
		memory:      uint32(values["m"]),
		iterations:  uint32(values["t"]),
		parallelism: uint8(values["p"]),
	}
	if parameters.memory < 19*1024 || parameters.memory > 1024*1024 ||
		parameters.iterations < 1 || parameters.iterations > 10 ||
		parameters.parallelism < 1 || parameters.parallelism > 16 {
		return hashParameters{}, nil, nil, errors.New("argon2 parameters outside safe bounds")
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) < 16 || len(salt) > 64 {
		return hashParameters{}, nil, nil, errors.New("invalid argon2 salt")
	}
	expected, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(expected) < 16 || len(expected) > 64 {
		return hashParameters{}, nil, nil, errors.New("invalid argon2 key")
	}
	return parameters, salt, expected, nil
}
