package producthandler

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
)

func ParseUUIDParam(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, errors.New("invalid UUID format")
	}
	return id, nil
}

func ParseIntParam(s string, paramName string) (int, error) {
	val, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("Invalid '%s' parameter. Must be an integer. Received %s", paramName, s)
	}
	return val, nil
}

func ParseBoolParam(s string, paramName string) (bool, error) {
	val, err := strconv.ParseBool(s)
	if err != nil {
		return false, fmt.Errorf("invalid '%s' parameter. Must be 'true' or 'false'. Received %s", paramName, s)
	}
	return val, nil
}

func ParseUUIDsFromQueryArray(paramValues []string, paramName string) ([]uuid.UUID, error) {
	if len(paramValues) == 0 {
		return []uuid.UUID{}, nil
	}

	uuids := make([]uuid.UUID, len(paramValues))
	for i, valStr := range paramValues {
		id, err := uuid.Parse(valStr)
		if err != nil {
			return nil, fmt.Errorf("invalid '%s' ID format. All IDs must be valid UUIDs", paramName)
		}
		uuids[i] = id
	}
	return uuids, nil
}
